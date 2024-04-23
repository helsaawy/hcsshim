//go:build windows

package computecore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"go.opencensus.io/trace"
	"golang.org/x/sys/windows"

	hcsschema "github.com/Microsoft/hcsshim/internal/hcs/schema2"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/oc"
	"github.com/sirupsen/logrus"
)

// TODO:
// - HcsGetOperationContext
// - HcsSetOperationContext
// - HcsGetComputeSystemFromOperation
// - HcsGetProcessFromOperation
// - HcsGetOperationResultAndProcessInfo
// - HcsWaitForOperationResultAndProcessInfo

// Handle to an HCSOperation on a compute system.
type HCSOperation windows.Handle

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

//	 HCS_OPERATION WINAPI
//	 HcsCreateOperation(
//	     _In_opt_ void*                    context
//	     _In_opt_ HCS_OPERATION_COMPLETION callback
//	     );
//
//sys hcsCreateOperation(context HCSContext, callback hcsOperationCompletionUintptr) (op HCSOperation, err error) = computecore.HcsCreateOperation?

func NewOperation(ctx context.Context, hcsCtx HCSContext, callback HCSOperationCompletion) (HCSOperation, error) {
	return createOperation(ctx, hcsCtx, callback.asCallback())
}

func NewEmptyOperation(ctx context.Context) (op HCSOperation, err error) {
	return createOperation(ctx, 0, 0)
}

func createOperation(ctx context.Context, hcsCtx HCSContext, callback hcsOperationCompletionUintptr) (op HCSOperation, err error) {
	_, span := oc.StartSpan(ctx, "computecore::HcsCreateOperation", oc.WithClientSpanKind)
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

func (op HCSOperation) Close() error {
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
func (op HCSOperation) Type() (HCSOperationType, error) {
	return hcsGetOperationType(op)
}

//	 UINT64 WINAPI
//	 HcsGetOperationId(
//	     _In_ HCS_OPERATION operation
//	     );
//
//sys hcsGetOperationID(operation HCSOperation) (id uint64, err error)= computecore.HcsGetOperationId?

// Returns the Id that uniquely identifies an operation.
func (op HCSOperation) ID() (uint64, error) {
	return hcsGetOperationID(op)
}

//	 HRESULT WINAPI
//	 HcsGetOperationResult(
//	     _In_ HCS_OPERATION operation,
//	     _Outptr_opt_ PWSTR* resultDocument
//	     );
//
//sys hcsGetOperationResult(operation HCSOperation, resultDocument **uint16) (hr error)= computecore.HcsGetOperationResult?

// GetOperationResult gets the result of the operation used to track an HCS function;
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
func (op HCSOperation) GetOperationResult(ctx context.Context) (result string, err error) {
	_, span := oc.StartSpan(ctx, "computecore::HcsGetOperationResult", oc.WithClientSpanKind)
	defer func() {
		if result != "" {
			span.AddAttributes(trace.StringAttribute("resultDocument", result))
		}
		oc.SetSpanStatus(span, err)
		span.End()
	}()
	span.AddAttributes(trace.StringAttribute("operation", op.String()))

	var resultp *uint16
	err = hcsGetOperationResult(op, &resultp)
	return processResults(ctx, bufferToString(resultp), err)
}

//	 HRESULT WINAPI
//	 HcsWaitForOperationResult(
//	     _In_ HCS_OPERATION operation,
//	     _In_ DWORD timeoutMs,
//	     _Outptr_opt_ PWSTR* resultDocument
//	     );
//
//sys hcsWaitForOperationResult(operation HCSOperation, timeoutMs uint32, resultDocument **uint16) (hr error) = computecore.HcsWaitForOperationResult?

// WaitForOperationResult waits for the operation to complete or the context to be cancelled.
//
// The maximum possible duration is [math.MaxUint32] milliseconds (approximately 50 days).
// If no context deadline is provided, the operation waits indefinitely.
//
// Note: if no deadline is provided, cancelling the context will not cancel the underlying HCS (wait) operation.
//
// See: [GetOperationResult].
func (op HCSOperation) WaitForOperationResult(ctx context.Context) (result string, err error) {
	ctx, span := oc.StartSpan(ctx, "computecore::HcsWaitForOperationResult", oc.WithClientSpanKind)
	defer func() {
		if result != "" {
			span.AddAttributes(trace.StringAttribute("resultDocument", result))
		}
		oc.SetSpanStatus(span, err)
		span.End()
	}()

	// While a timeout on ctx will cancel waiting for the operation result, the underlying HCS operation
	// will still be ongoing (since HcsCancelOperation is not yet implemented).
	// We use the context timeout (if one is set), as the timeout for the HCS wait syscall as well,
	// rather than expose two competing timeouts (the context and HCS parameter timeouts).
	milli := contextTimeoutMs(ctx)
	span.AddAttributes(
		trace.StringAttribute("operation", op.String()),
		trace.Int64Attribute("timeoutMs", int64(milli)))

	// Don't write to return variables (result, err) directly from goroutine:
	// that can cause a data race if this function exits before the goroutine.
	//
	// Don't use a `chan error`: the `<-hcsWaitForOperationResult` will block and cause the
	// goroutine to hang indefinitely if this function exits first (since nothing will read from the channel).
	//
	// Don't use operation callback to call `close(done)`: it will fail if the compute system already has a callback

	var opErr error
	var resultp *uint16
	done := make(chan struct{})
	go func() {
		defer close(done)

		log.G(ctx).Trace("waiting on operation")
		opErr = hcsWaitForOperationResult(op, milli, &resultp)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return processResults(ctx, bufferToString(resultp), opErr)
}

// if err != nil, try to parse result as an [hcsschema.ResultError].
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
func (op HCSOperation) SetOperationCallback(ctx context.Context, hcsCtx HCSContext, callback HCSOperationCompletion) (err error) {
	_, span := oc.StartSpan(ctx, "computecore::HcsSetOperationCallback", oc.WithClientSpanKind)
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
