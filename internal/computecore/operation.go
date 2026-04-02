//go:build windows

package computecore

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/windows"
)

// HCSContext corresponds to a `void* context` parameter that allows for an arbitrary payload
// containing compute system-, process-, or operation-specific data to be passed to callbacks.
//
// It is not compatible with [context.HCSContext].
type HCSContext uintptr

// HCSOperation is the handle associated with an operation on a compute system.
type HCSOperation windows.Handle

const invalidHCSOperation = HCSOperation(windows.InvalidHandle)

func (op HCSOperation) String() string {
	return "0x" + strconv.FormatInt(int64(op), 16)
}

//go:generate go run golang.org/x/tools/cmd/stringer -type=HCSOperationType -trimprefix=OperationType operation.go

// HCSOperationType is the type of an operation, returned by hcsGetOperationType.
//
// See [documentation] for more info.
//
// [documentation]: https://learn.microsoft.com/en-us/virtualization/api/hcs/reference/hcs_operation_type
type HCSOperationType int32

const (
	OperationTypeNone                 = HCSOperationType(-1)
	OperationTypeEnumerate            = HCSOperationType(0)
	OperationTypeCreate               = HCSOperationType(1)
	OperationTypeStart                = HCSOperationType(2)
	OperationTypeShutdown             = HCSOperationType(3)
	OperationTypePause                = HCSOperationType(4)
	OperationTypeResume               = HCSOperationType(5)
	OperationTypeSave                 = HCSOperationType(6)
	OperationTypeTerminate            = HCSOperationType(7)
	OperationTypeModify               = HCSOperationType(8)
	OperationTypeGetProperties        = HCSOperationType(9)
	OperationTypeCreateProcess        = HCSOperationType(10)
	OperationTypeSignalProcess        = HCSOperationType(11)
	OperationTypeGetProcessInfo       = HCSOperationType(12)
	OperationTypeGetProcessProperties = HCSOperationType(13)
	OperationTypeModifyProcess        = HCSOperationType(14)
	OperationTypeCrash                = HCSOperationType(15)
)

var operationCallback = HCSCallback(windows.NewCallback(nil)) // TODO

var opManager *operationManager

func init() {
	opManager = &operationManager{
		ops: make(map[HCSContext]*operation),
	}
}

type operationManager struct {
	// counter for HCSContext values
	counter atomic.Uintptr

	opsM sync.RWMutex
	// map of all operations
	ops map[HCSContext]*operation
}

// createOperation creates a new operation to track an HCS function call
func (m *operationManager) createOperation(ctx context.Context, hcsOp HCSOperation) (_ *operation, err error) {
	opCtx := m.newContext()

	op, err := newOperation(ctx, opCtx, hcsOp)
	if err != nil {
		return nil, err
	}

	m.opsM.Lock()
	m.ops[opCtx] = op
	m.opsM.Unlock()

	return op, nil
}

func (m *operationManager) newContext() HCSContext { return HCSContext(m.counter.Add(1)) }

type operationStartFn func(context.Context, HCSOperation) error
type operationEndFn func(context.Context, HCSOperation) (json.RawMessage, error)

// run synchronously executes an operation, waiting until its completion
func (m *operationManager) run(ctx context.Context, hcsOp HCSOperation, start operationStartFn, end operationEndFn) error {
	// TODO: create span with add operation ID and type as attributes?

	if start == nil {
		return fmt.Errorf("invalid operationStartFn")
	}
	if end == nil {
		end = HcsGetOperationResult
	}

	op, err := m.createOperation(ctx, hcsOp)
	if err != nil {
		return fmt.Errorf("create new operation: %w", err)
	}

	if err := op.start(ctx, start); err != nil {
		return fmt.Errorf("start operation: %w", err)
	}

	if err = op.wait(ctx); err != nil {
		return fmt.Errorf("wait on operation: %w", err)
	}

	// TODO:
	// operationEndFn
	// parse resultError
	return nil
}

// TODO: wrap HCSOperation* functions and add mock provider
type operationProvider interface {
}

// hcsOperationPool of HCSOperations to reuse
//
// [sync.Pool.New] doesn't return errors associated with creating new resources, so
// create them out of band in order to be able to propagate errors to caller.
var hcsOperationPool = &sync.Pool{}

// operation is the statemachine controller for handling operation state.
type operation struct {
	ctx HCSContext

	stateM sync.RWMutex
	state  operationState
}

func newOperation(ctx context.Context, opCtx HCSContext, hcsOp HCSOperation) (_ *operation, err error) {
	// if the caller provided their own HCSOperation
	owned := hcsOp == 0 || hcsOp == invalidHCSOperation
	if !owned {
		// given an operation from somewhere, so we couldn't set the callback before
		if err := HcsSetOperationCallback(ctx, hcsOp, opCtx, operationCallback); err != nil {
			return nil, err
		}
	} else {
		var ok bool
		hcsOp, ok = hcsOperationPool.Get().(HCSOperation)
		if !ok || hcsOp == 0 || hcsOp == invalidHCSOperation {
			hcsOp, err = HcsCreateOperation(ctx, opCtx, operationCallback)
			if err != nil {
				return nil, err
			}
		}
		// update the operation context
		if err = HcsSetOperationContext(ctx, hcsOp, opCtx); err != nil {
			return nil, err
		}
	}

	return &operation{
		ctx: opCtx,
		state: &operationCreated{
			op:      hcsOp,
			opOwned: owned,
		},
	}, nil
}

// start the operation using the provided function and update the state accordingly.
func (op *operation) start(ctx context.Context, fn operationStartFn) error {
	op.stateM.Lock()
	defer op.stateM.Unlock()

	switch st := op.state.(type) {
	case *operationCreated:
		var newSt operationState

		err := fn(ctx, st.op)
		if err != nil {
			newSt = &operationFinished{
				err: err,
			}
			if st.opOwned {
				hcsOperationPool.Put(st.op)
			}
		} else {
			newSt = &operationStarted{
				op:        st.op,
				ownedOp:   st.opOwned,
				waitBlock: newBlock(),
			}
		}

		op.state = newSt
		return err
	default:
		return fmt.Errorf("invalid operation state %s: %w", st.stateName(), ErrOperationAlreadyStarted)
	}
}

func (op *operation) wait(ctx context.Context) error {
	blk, err := func() (*block, error) {
		op.stateM.RLock()
		defer op.stateM.RUnlock()

		switch st := op.state.(type) {
		case *operationCancelled, *operationFinished:
			return nil, nil
		case *operationStarted:
			return st.waitBlock, nil
		default:
			return nil, fmt.Errorf("invalid operation state %s: %w", st.stateName(), ErrOperationNotStarted)
		}
	}()

	if err != nil || blk == nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-blk.done():
		return nil
	}
}

// enum type for operationState
type operationState interface {
	stateName() string
}

type operationCreated struct {
	op      HCSOperation
	opOwned bool
}

func (x *operationCreated) stateName() string { return "CREATED" }

type operationStarted struct {
	op        HCSOperation
	ownedOp   bool
	waitBlock *block
}

func (x *operationStarted) stateName() string { return "STARTED" }

type operationFinished struct {
	// true if the operation failed to start, otherwise the operation failed after starting correctly.
	started bool
	err     error
}

func (x *operationFinished) stateName() string { return "FINISHED" }

type operationCancelled struct {
	err error
}

func (x *operationCancelled) stateName() string { return "CANCELLED" }

type block struct {
	// TODO: noCopy

	ch chan struct{}
	// we can't tell if a chan has been closed without attempting to wait on it
	// and closing a closed channel panics.
	// so use closeFn to guard against incorrect/concurrent channel close.
	closeFn func()
}

func newBlock() *block {
	ch := make(chan struct{})
	return &block{
		ch:      ch,
		closeFn: sync.OnceFunc(func() { close(ch) }),
	}
}

func (b *block) done() <-chan struct{} {
	return b.ch
}
