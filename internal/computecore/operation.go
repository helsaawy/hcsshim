//go:build windows

package computecore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"go.opencensus.io/trace"
	"golang.org/x/sys/windows"

	hcsschema "github.com/Microsoft/hcsshim/internal/hcs/schema2"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/oc"
	"github.com/sirupsen/logrus"
)

// Note: we don't have a good way to cancel attempting to lock the mutext, so these
// operations may block.
// Callers should handle checking against context timeout.

// Since the `done <-struct{}` channel will change as new operations are started, don't expose
// that directly to users.

type Operation struct {
	// m locks st, preventing writes during ongoing operations.
	m sync.RWMutex
	// current operation state.
	st operationState
}

func NewEmptyOperation(ctx context.Context) (*Operation, error) {
	return NewOperation(ctx, 0, nil)
}

func NewOperation(ctx context.Context, hcsCtx HCSContext, callback HCSOperationCompletion) (*Operation, error) {
	h, err := createOperation(ctx, hcsCtx, callback.asCallback())
	if err != nil {
		return nil, err
	}

	op := &Operation{
		st: &operationCreated{h: h},
	}
	return op, nil
}

// unsafe: `op != nil` && must hold `op.m`
func (op *Operation) valid() bool {
	return op.st != nil && op.st.operation().valid()
}

func (op *Operation) run(
	ctx context.Context,
	f func(hcsOperation) error,
) error {
	// start and wait on operation in a separate go routine, since
	// mutex operation can block.
	errCh := make(chan error)
	go func() {
		defer close(errCh)

		if err := op.Start(ctx, f, contextTimeoutMs(ctx)); err != nil {
			select {
			case errCh <- err:
			case <-ctx.Done():
				// context was cancelled, and parent function returned
				// exit so go routine doesn't leak
			}
			return
		}

		// todo: wait on result.
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		// context was cancelled, and parent function returned
		// exit so go routine doesn't leak
		return ctx.Err()
	}
}

// Start an operation via the function f.
// If successfully started, the underlying HCS operation will timeout after timeoutMs milliseconds.
//
// Note: there is not way to cancel attempting to lock op.m, so this operations may block
// even after ctx is cancelled.
// Additionally, using timeout associated with ctx will leak the context, which is undesired
// if the context is to be associated with the operation start, and not the subsequent wait.
//
// See [waitBackground] for timeout details..
func (op *Operation) Start(
	ctx context.Context,
	f func(hcsOperation) error,
	timeoutMs uint32,
) (err error) {
	ctx, span := oc.StartSpan(ctx, operationSpanName("Start"), oc.WithClientSpanKind)
	defer func() {
		oc.SetSpanStatus(span, err)
		span.End()
	}()

	if op == nil {
		return fmt.Errorf("nil operation: %w", windows.ERROR_INVALID_HANDLE)
	}

	op.m.Lock()
	defer op.m.Unlock()

	if !op.valid() {
		return fmt.Errorf("invalid operation handle: %w", windows.ERROR_INVALID_HANDLE)
	}

	span.AddAttributes(
		trace.StringAttribute("operation", op.st.operation().String()),
		trace.StringAttribute("state", op.st.String()),
		trace.Int64Attribute("timeoutMs", int64(timeoutMs)))

	switch op.st.(type) {
	case *operationStarted:
		return fmt.Errorf("ongoing operation: %w", HCS_E_OPERATION_ALREADY_STARTED)
	case *operationClosed:
		return fmt.Errorf("operation already closed: %w", windows.ERROR_HANDLES_CLOSED)
	default:
	}

	h := op.st.operation()

	if err := f(h); err != nil {
		return err
	}

	done := make(chan struct{})
	st := &operationStarted{done: done}

	go func(ctx context.Context) {
		defer close(done)
		st := &operationCompleted{
			h: h,
		}

		// WaitForOperationResult should (attempt to) parse result as [hcsschema.ResultError]
		// so just return results
		st.result, st.err = h.waitBackground(ctx, timeoutMs)

		// lock operation to change state
		op.m.Lock()
		defer op.m.Unlock()

		switch op.st.(type) {
		case *operationStarted:
			op.st = st
		default:
			// somehow someone else change the state, even after we started it
			// TODO: log these situations
		}

	}(context.WithoutCancel(ctx))

	op.st = st
	return nil
}

func (op *Operation) Wait(ctx context.Context) (result string, err error) {
	ctx, span := oc.StartSpan(ctx, operationSpanName("Wait"), oc.WithClientSpanKind)
	defer func() {
		if result != "" {
			span.AddAttributes(trace.StringAttribute("resultDocument", result))
		}
		oc.SetSpanStatus(span, err)
		span.End()
	}()

	// TODO

	// span.AddAttributes(
	// 	trace.StringAttribute("operation", op.String()),
	// 	trace.Int64Attribute("timeoutMs", int64(timeoutMs)))

	return "", nil
}

func (op *Operation) Pending(context.Context) bool {
	// TODO
	return false
}

func (op *Operation) Closed(context.Context) bool {
	// TODO
	return false
}

func (op *Operation) Close(ctx context.Context) (err error) {
	ctx, span := oc.StartSpan(ctx, operationSpanName("Close"), oc.WithClientSpanKind)
	defer func() {
		oc.SetSpanStatus(span, err)
		span.End()
	}()

	if op == nil {
		return fmt.Errorf("nil operation: %w", windows.ERROR_INVALID_HANDLE)
	}

	op.m.Lock()
	defer op.m.Unlock()

	span.AddAttributes(trace.StringAttribute("state", op.st.String()))

	switch op.st.(type) {
	case *operationClosed:
		return nil
	default:
	}

	if !op.valid() {
		return fmt.Errorf("invalid operation handle: %w", windows.ERROR_INVALID_HANDLE)
	}

	h := op.st.operation()
	// replace the state regardless of Close() return value
	op.st = &operationClosed{}
	return h.close()
}

// we want enum structs, but have to hack it in :'(

// the value of hcsOperation must be constant across state transitions

type operationState interface {
	fmt.Stringer
	operation() hcsOperation // unsafe: `op != nil` && must hold `op.m`
}

type operationCreated struct {
	h hcsOperation
}

var _ operationState = new(operationCreated)

func (op *operationCreated) operation() hcsOperation { return op.h }
func (*operationCreated) String() string             { return "Idle" }

type operationStarted struct {
	h    hcsOperation
	done <-chan struct{}
}

var _ operationState = new(operationStarted)

func (op *operationStarted) operation() hcsOperation { return op.h }
func (*operationStarted) String() string             { return "Started" }

type operationCompleted struct {
	h      hcsOperation
	result string
	err    error
}

var _ operationState = new(operationCompleted)

func (op *operationCompleted) operation() hcsOperation { return op.h }
func (*operationCompleted) String() string             { return "Completed" }

type operationClosed struct{}

var _ operationState = new(operationClosed)

func (*operationClosed) operation() hcsOperation { return hcsOperation(windows.InvalidHandle) }
func (*operationClosed) String() string          { return "Close" }

// TODO:
// - HcsGetOperationContext
// - HcsSetOperationContext
// - HcsGetComputeSystemFromOperation
// - HcsGetProcessFromOperation
// - HcsGetOperationResultAndProcessInfo
// - HcsWaitForOperationResultAndProcessInfo

// Handle to an operation on an HCS compute system, process, or other resource.
type hcsOperation windows.Handle

func (op hcsOperation) valid() bool { return validHandle(windows.Handle(op)) }

func (op hcsOperation) String() string {
	// treat the handle as a unique ID for the operation, since the operation ID (from [HcsGetOperationId])
	// may change across operation invovations.
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

//	 HCS_OPERATION WINAPI
//	 HcsCreateOperation(
//	     _In_opt_ void*                    context
//	     _In_opt_ HCS_OPERATION_COMPLETION callback
//	     );
//
//sys hcsCreateOperation(context HCSContext, callback hcsOperationCompletionUintptr) (op HCSOperation, err error) = computecore.HcsCreateOperation?

func createOperation(ctx context.Context, hcsCtx HCSContext, callback hcsOperationCompletionUintptr) (op hcsOperation, err error) {
	_, span := oc.StartSpan(ctx, computecoreSpanName("HcsCreateOperation"), oc.WithClientSpanKind)
	defer func() {
		span.AddAttributes(trace.StringAttribute("operation", op.String()))
		oc.SetSpanStatus(span, err)
		span.End()
	}()
	span.AddAttributes(
		trace.Int64Attribute("context", int64(hcsCtx)),
		trace.Int64Attribute("callback", int64(callback)),
	)

	return hcsCreateOperation(hcsCtx, callback)
}

//	 void WINAPI
//	 HcsCloseOperation(
//	     _In_ HCS_OPERATION operation
//	     );
//
//sys hcsCloseOperation(operation HCSOperation) = computecore.HcsCloseOperation?

func (op hcsOperation) close() error {
	// should only return an error if ComputeCore.dll isn't found ...
	return hcsCloseOperation(op)
}

//	 HCS_OPERATION_TYPE WINAPI
//	 HcsGetOperationType(
//	     _In_ HCS_OPERATION operation
//	     );
//
//sys hcsGetOperationType(operation HCSOperation) (t HCSOperationType, err error) = computecore.HcsGetOperationType?

// Get the type of the operation, this corresponds to the API call the operation was issued with.
func (op hcsOperation) operationType() (HCSOperationType, error) {
	return hcsGetOperationType(op)
}

//	 UINT64 WINAPI
//	 HcsGetOperationId(
//	     _In_ HCS_OPERATION operation
//	     );
//
//sys hcsGetOperationID(operation HCSOperation) (id uint64, err error)= computecore.HcsGetOperationId?

// Returns the Id that uniquely identifies an operation.
func (op hcsOperation) id() (uint64, error) {
	return hcsGetOperationID(op)
}

//	 HRESULT WINAPI
//	 HcsGetOperationResult(
//	     _In_ HCS_OPERATION operation,
//	     _Outptr_opt_ PWSTR* resultDocument
//	     );
//
//sys hcsGetOperationResult(operation HCSOperation, resultDocument **uint16) (hr error)= computecore.HcsGetOperationResult?

// func (op hcsOperation) getOperationResult(ctx context.Context) (result string, err error) {
// 	_, span := oc.StartSpan(ctx, computecoreSpanName("HcsGetOperationResult"), oc.WithClientSpanKind)
// 	defer func() {
// 		if result != "" {
// 			span.AddAttributes(trace.StringAttribute("resultDocument", result))
// 		}
// 		oc.SetSpanStatus(span, err)
// 		span.End()
// 	}()
// 	span.AddAttributes(trace.StringAttribute("operation", op.String()))

// 	var resultp *uint16
// 	err = hcsGetOperationResult(op, &resultp)
// 	return processResults(ctx, bufferToString(resultp), err)
// }

//	 HRESULT WINAPI
//	 HcsWaitForOperationResult(
//	     _In_ HCS_OPERATION operation,
//	     _In_ DWORD timeoutMs,
//	     _Outptr_opt_ PWSTR* resultDocument
//	     );
//
//sys hcsWaitForOperationResult(operation HCSOperation, timeoutMs uint32, resultDocument **uint16) (hr error) = computecore.HcsWaitForOperationResult?

// WaitForOperationResult synchronously waits for and returns the result of an HCS operation to complete;
// optionally returns a JSON document associated to such tracked operation.
//
// On failure, it will attempt to parse the (optional) error JSON document as a [hcsschema.ResultError];
// it's not guaranteed to be always returned and depends on the function call the operation was tracking.
//
// Returned errors:
//   - [HCS_E_OPERATION_NOT_STARTED] if the operation has not been started.
//     This is expected when the operation has not been used yet in an HCS function that expects an HCS_OPERATION handle.
//   - [HCS_E_OPERATION_PENDING] if the operation is still in progress and hasn't been completed, regardless of success or failure
//   - Any other value if the operation completed with failures.
//     The returned HRESULT is dependent on the HCS function thas was being tracked.
//
// The maximum possible duration is [math.MaxUint32] milliseconds (approximately 50 days).
//
// Note: The provided context is only used for logging; cancellations and timeouts are ignored.
// Use timeoutMs to cancel the wait pre-emptively.
func (op hcsOperation) waitBackground(ctx context.Context, timeoutMs uint32) (string, error) {
	// Could extract the timeout from the context directly (via [contextTimeoutMs]), but
	// then this function would only respect context deadlines and not cancellation.
	//
	// Could start [hcsWaitForOperationResult] in a go routine and select on that as well and
	// `<-ctx.Done()`, but since this function will be called from a go-routine regardless,
	// might as well save on a nested go routine.
	//
	// Also, since [HcsCancelOperation] is not yet implemented (see below), having cancellations
	// cancel this and leak the underlying operation, which is also unsatisfactory.
	//
	// Also, don't use operation callback to handle context cancellation and processing results:
	// it will fail if the process/compute system associated with the operation already has a callback

	var resultp *uint16
	err := hcsWaitForOperationResult(op, timeoutMs, &resultp)

	return processResults(ctx, bufferToString(resultp), err)
}

// Try to process operation results.
// If err != nil, try to parse result as an [hcsschema.ResultError].
func processResults(ctx context.Context, result string, err error) (string, error) {
	if err == nil || result == "" {
		// if there is not error or if the result document is empty, do nothing
		return result, err
	}

	re := hcsschema.ResultError{}
	if jErr := json.Unmarshal([]byte(result), &re); jErr != nil {
		// if unmarshalling fails, ignore it and move on
		// not really a worthy enough error to raise it or log at Info or above
		log.G(ctx).WithError(jErr).Debugf("failed to unmarshal result as a %T", re)
		return result, err
	}

	// resultDocument will be set to "", so log it here in case its needed to debug error parsing
	log.G(ctx).WithField("resultDocument", result).Tracef("parsed operation result document as %T", re)

	// err should be (🤞) the same as `re.Error_`, but validate just in case
	if eno := windows.Errno(0x0); errors.As(err, &eno) {
		eno64 := uint64(eno)
		// convert to uint32 first to prevent leftpadding
		hr64 := uint64(uint32(re.Error_))
		if hr64 != eno64 {
			log.G(ctx).WithFields(logrus.Fields{
				"operationError": strconv.FormatUint(eno64, 16),
				"resultError":    strconv.FormatUint(hr64, 16),
			}).Warning("error mismatch between operation error and result error; overriding result error value")
			re.Error_ = int32(math.MaxUint32 & eno)
			re.ErrorMessage = eno.Error()
		}
	} else {
		log.G(ctx).WithFields(logrus.Fields{
			logrus.ErrorKey: err,
			"expectedType":  fmt.Sprintf("%T", eno),
			"receivedType":  fmt.Sprintf("%T", err),
		}).Warning("unexpected expected error type")
	}

	// return ResultError instead of err
	return "", &re
}

// returns the context timeout in milliseconds, as a uint32, so it can be used with HcsWaitForOperationResult.
func contextTimeoutMs(ctx context.Context) uint32 {
	deadline, ok := ctx.Deadline()
	if !ok {
		// no deadline set, wait infinitely
		return math.MaxUint32
	}

	timeout := time.Until(deadline).Milliseconds()
	if timeout >= math.MaxUint32 {
		// duration overflows; wait for 1 millisecond less than maximum possible value
		return math.MaxUint32 - 1
	}
	if timeout < 0 {
		// context is already cancelled; use a zero timeout
		return 0
	}
	return uint32(timeout)
}

//	 HRESULT WINAPI
//	 HcsSetOperationCallback(
//	     _In_ HCS_OPERATION operation,
//	     _In_opt_ const void* context,
//	     _In_ HCS_OPERATION_COMPLETION callback
//	     );
//
//sys hcsSetOperationCallback(operation HCSOperation, context HCSContext, callback hcsOperationCompletionUintptr) (hr error) = computecore.HcsSetOperationCallback?

// Its best not to use [SetOperationCallback], since it is possible the compute system already has a callback assigned.
func (op hcsOperation) setOperationCallback(ctx context.Context, hcsCtx HCSContext, callback HCSOperationCompletion) (err error) {
	_, span := oc.StartSpan(ctx, computecoreSpanName("HcsSetOperationCallback"), oc.WithClientSpanKind)
	defer func() {
		oc.SetSpanStatus(span, err)
		span.End()
	}()
	ptr := callback.asCallback()
	span.AddAttributes(
		trace.StringAttribute("operation", op.String()),
		trace.Int64Attribute("context", int64(hcsCtx)),
		trace.Int64Attribute("callback", int64(ptr)),
	)

	return hcsSetOperationCallback(op, hcsCtx, ptr)
}

// TODO: expose this when its implemented:
// https://learn.microsoft.com/en-us/virtualization/api/hcs/reference/hcscanceloperation#remarks
//
//	 HRESULT WINAPI
//	 HcsCancelOperation (
//	     _In_ HCS_OPERATION operation
//	     );
//
//sys hcsCancelOperation(operation HCSOperation) (hr error) = computecore.HcsCancelOperation?

func operationSpanName(names ...string) string {
	return computecoreSpanName(append([]string{"Operation"}, names...)...)
}
