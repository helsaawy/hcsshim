//go:build windows

package computecore

import (
	"context"
	"encoding/json"
	"time"
	"unsafe"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
	"golang.org/x/sys/windows"

	"github.com/Microsoft/hcsshim/internal/interop"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/Microsoft/hcsshim/internal/oc"
	"github.com/Microsoft/hcsshim/internal/timeout"
)

//go:generate go tool github.com/Microsoft/go-winio/tools/mkwinsyscall -output zsyscall_windows.go computecore.go

// Operation management
//sys hcsCreateOperation(context HCSContext, callback HCSCallback) (operation HCSOperation, err error) = computecore.HcsCreateOperation?
//sys hcsCreateOperationWithNotifications(eventTypes uint32, context HCSContext, callback HCSCallback) (operation HCSOperation, err error) = computecore.HcsCreateOperationWithNotifications?
//sys hcsCloseOperation(operation HCSOperation) = computecore.HcsCloseOperation
//sys hcsGetOperationContext(operation HCSOperation) (context HCSContext) = computecore.HcsGetOperationContext
//sys hcsSetOperationContext(operation HCSOperation, context HCSContext) (hr error) = computecore.HcsSetOperationContext?
//sys hcsGetComputeSystemFromOperation(operation HCSOperation) (computeSystem HCSSystem) = computecore.HcsGetComputeSystemFromOperation
//sys hcsGetProcessFromOperation(operation HCSOperation) (process HCSProcess) = computecore.HcsGetProcessFromOperation
//sys hcsGetOperationType(operation HCSOperation) (operationType int32) = computecore.HcsGetOperationType
//sys hcsGetOperationId(operation HCSOperation) (operationId uint64) = computecore.HcsGetOperationId
//sys hcsGetOperationResult(operation HCSOperation, resultDocument **uint16) (hr error) = computecore.HcsGetOperationResult?
//sys hcsGetOperationResultAndProcessInfo(operation HCSOperation, processInformation *HCSProcessInformation, resultDocument **uint16) (hr error) = computecore.HcsGetOperationResultAndProcessInfo?
//sys hcsAddResourceToOperation(operation HCSOperation, resourceType uint32, uri string, handle windows.Handle) (hr error) = computecore.HcsAddResourceToOperation?
//sys hcsGetProcessorCompatibilityFromSavedState(runtimeFileName string, processorFeaturesString **uint16) (hr error) = computecore.HcsGetProcessorCompatibilityFromSavedState?
//sys hcsWaitForOperationResult(operation HCSOperation, timeoutMs uint32, resultDocument **uint16) (hr error) = computecore.HcsWaitForOperationResult?
//sys hcsWaitForOperationResultAndProcessInfo(operation HCSOperation, timeoutMs uint32, processInformation *HCSProcessInformation, resultDocument **uint16) (hr error) = computecore.HcsWaitForOperationResultAndProcessInfo?
//sys hcsSetOperationCallback(operation HCSOperation, context HCSContext, callback HCSCallback) (hr error) = computecore.HcsSetOperationCallback?
//sys hcsCancelOperation(operation HCSOperation) (hr error) = computecore.HcsCancelOperation?
//sys hcsGetOperationProperties(operation HCSOperation, options string, resultDocument **uint16) (hr error) = computecore.HcsGetOperationProperties?

// Compute system lifecycle
//sys hcsEnumerateComputeSystems(query string, operation HCSOperation) (hr error) = computecore.HcsEnumerateComputeSystems?
//sys hcsEnumerateComputeSystemsInNamespace(idNamespace string, query string, operation HCSOperation) (hr error) = computecore.HcsEnumerateComputeSystemsInNamespace?
//sys hcsCreateComputeSystem(id string, configuration string, operation HCSOperation, securityDescriptor unsafe.Pointer, computeSystem *HCSSystem) (hr error) = computecore.HcsCreateComputeSystem?
//sys hcsCreateComputeSystemInNamespace(idNamespace string, id string, configuration string, operation HCSOperation, options unsafe.Pointer, computeSystem *HCSSystem) (hr error) = computecore.HcsCreateComputeSystemInNamespace?
//sys hcsOpenComputeSystem(id string, requestedAccess uint32, computeSystem *HCSSystem) (hr error) = computecore.HcsOpenComputeSystem?
//sys hcsOpenComputeSystemInNamespace(idNamespace string, id string, requestedAccess uint32, computeSystem *HCSSystem) (hr error) = computecore.HcsOpenComputeSystemInNamespace?
//sys hcsCloseComputeSystem(computeSystem HCSSystem) = computecore.HcsCloseComputeSystem
//sys hcsStartComputeSystem(computeSystem HCSSystem, operation HCSOperation, options string) (hr error) = computecore.HcsStartComputeSystem?
//sys hcsShutDownComputeSystem(computeSystem HCSSystem, operation HCSOperation, options string) (hr error) = computecore.HcsShutDownComputeSystem?
//sys hcsTerminateComputeSystem(computeSystem HCSSystem, operation HCSOperation, options string) (hr error) = computecore.HcsTerminateComputeSystem?
//sys hcsCrashComputeSystem(computeSystem HCSSystem, operation HCSOperation, options string) (hr error) = computecore.HcsCrashComputeSystem?
//sys hcsPauseComputeSystem(computeSystem HCSSystem, operation HCSOperation, options string) (hr error) = computecore.HcsPauseComputeSystem?
//sys hcsResumeComputeSystem(computeSystem HCSSystem, operation HCSOperation, options string) (hr error) = computecore.HcsResumeComputeSystem?
//sys hcsSaveComputeSystem(computeSystem HCSSystem, operation HCSOperation, options string) (hr error) = computecore.HcsSaveComputeSystem?
//sys hcsGetComputeSystemProperties(computeSystem HCSSystem, operation HCSOperation, propertyQuery string) (hr error) = computecore.HcsGetComputeSystemProperties?
//sys hcsModifyComputeSystem(computeSystem HCSSystem, operation HCSOperation, configuration string, identity windows.Handle) (hr error) = computecore.HcsModifyComputeSystem?
//sys hcsWaitForComputeSystemExit(computeSystem HCSSystem, timeoutMs uint32, result **uint16) (hr error) = computecore.HcsWaitForComputeSystemExit?
//sys hcsSetComputeSystemCallback(computeSystem HCSSystem, callbackOptions uint32, context HCSContext, callback HCSCallback) (hr error) = computecore.HcsSetComputeSystemCallback?

// Live migration
//sys hcsInitializeLiveMigrationOnSource(computeSystem HCSSystem, operation HCSOperation, options string) (hr error) = computecore.HcsInitializeLiveMigrationOnSource?
//sys hcsStartLiveMigrationOnSource(computeSystem HCSSystem, operation HCSOperation, options string) (hr error) = computecore.HcsStartLiveMigrationOnSource?
//sys hcsStartLiveMigrationTransfer(computeSystem HCSSystem, operation HCSOperation, options string) (hr error) = computecore.HcsStartLiveMigrationTransfer?
//sys hcsFinalizeLiveMigration(computeSystem HCSSystem, operation HCSOperation, options string) (hr error) = computecore.HcsFinalizeLiveMigration?

// Process lifecycle
//sys hcsCreateProcess(computeSystem HCSSystem, processParameters string, operation HCSOperation, securityDescriptor unsafe.Pointer, process *HCSProcess) (hr error) = computecore.HcsCreateProcess?
//sys hcsOpenProcess(computeSystem HCSSystem, pid uint32, requestedAccess uint32, process *HCSProcess) (hr error) = computecore.HcsOpenProcess?
//sys hcsCloseProcess(process HCSProcess) = computecore.HcsCloseProcess
//sys hcsTerminateProcess(process HCSProcess, operation HCSOperation, options string) (hr error) = computecore.HcsTerminateProcess?
//sys hcsSignalProcess(process HCSProcess, operation HCSOperation, options string) (hr error) = computecore.HcsSignalProcess?
//sys hcsGetProcessInfo(process HCSProcess, operation HCSOperation) (hr error) = computecore.HcsGetProcessInfo?
//sys hcsGetProcessProperties(process HCSProcess, operation HCSOperation, propertyQuery string) (hr error) = computecore.HcsGetProcessProperties?
//sys hcsModifyProcess(process HCSProcess, operation HCSOperation, settings string) (hr error) = computecore.HcsModifyProcess?
//sys hcsSetProcessCallback(process HCSProcess, callbackOptions uint32, context HCSContext, callback HCSCallback) (hr error) = computecore.HcsSetProcessCallback?
//sys hcsWaitForProcessExit(process HCSProcess, timeoutMs uint32, result **uint16) (hr error) = computecore.HcsWaitForProcessExit?

// Service
//sys hcsGetServiceProperties(propertyQuery string, result **uint16) (hr error) = computecore.HcsGetServiceProperties?
//sys hcsModifyServiceSettings(settings string, result **uint16) (hr error) = computecore.HcsModifyServiceSettings?
//sys hcsSubmitWerReport(settings string) (hr error) = computecore.HcsSubmitWerReport?

// File and VM access
//sys hcsCreateEmptyGuestStateFile(guestStateFilePath string) (hr error) = computecore.HcsCreateEmptyGuestStateFile?
//sys hcsCreateEmptyRuntimeStateFile(runtimeStateFilePath string) (hr error) = computecore.HcsCreateEmptyRuntimeStateFile?
//sys hcsGrantVmAccess(vmId string, filePath string) (hr error) = computecore.HcsGrantVmAccess?
//sys hcsRevokeVmAccess(vmId string, filePath string) (hr error) = computecore.HcsRevokeVmAccess?
//sys hcsGrantVmGroupAccess(filePath string) (hr error) = computecore.HcsGrantVmGroupAccess?
//sys hcsRevokeVmGroupAccess(filePath string) (hr error) = computecore.HcsRevokeVmGroupAccess?

// errVmcomputeOperationPending is an error encountered when the operation is being completed asynchronously
const errVmcomputeOperationPending = windows.Errno(0xC0370103)

// HCSSystem is the handle associated with a created compute system.
type HCSSystem windows.Handle

// HCSProcess is the handle associated with a created process in a compute
// system.
type HCSProcess windows.Handle

// HCSCallback is the handle associated with the function to call when events
// occur.
type HCSCallback windows.Handle

// HCSProcessInformation is the structure used when creating or getting process
// info.
type HCSProcessInformation struct {
	// ProcessID is the pid of the created process.
	ProcessID uint32
	_         uint32 // reserved padding
	// StdInput is the handle associated with the stdin of the process.
	StdInput windows.Handle
	// StdOutput is the handle associated with the stdout of the process.
	StdOutput windows.Handle
	// StdError is the handle associated with the stderr of the process.
	StdError windows.Handle
}

func execute(ctx context.Context, timeout time.Duration, f func() error) error {
	now := time.Now()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	deadline, ok := ctx.Deadline()
	trueTimeout := timeout
	if ok {
		trueTimeout = deadline.Sub(now)
		log.G(ctx).WithFields(logrus.Fields{
			logfields.Timeout: trueTimeout,
			"desiredTimeout":  timeout,
		}).Trace("Executing syscall with deadline")
	}

	done := make(chan error, 1)
	go func() {
		done <- f()
	}()
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			log.G(ctx).WithField(logfields.Timeout, trueTimeout).
				Warning("Syscall did not complete within operation timeout. This may indicate a platform issue. " +
					"If it appears to be making no forward progress, obtain the stacks and see if there is a syscall " +
					"stuck in the platform API for a significant length of time.")
		}
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// Operation management

func HcsCreateOperation(ctx context.Context, callbackContext HCSContext, callback HCSCallback) (operation HCSOperation, hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsCreateOperation")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()

	return operation, execute(ctx, timeout.SyscallWatcher, func() error {
		var err error
		operation, err = hcsCreateOperation(callbackContext, callback)
		return err
	})
}

func HcsCreateOperationWithNotifications(ctx context.Context, eventTypes uint32, callbackContext HCSContext, callback HCSCallback) (operation HCSOperation, hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsCreateOperationWithNotifications")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()

	return operation, execute(ctx, timeout.SyscallWatcher, func() error {
		var err error
		operation, err = hcsCreateOperationWithNotifications(eventTypes, callbackContext, callback)
		return err
	})
}

func HcsCloseOperation(ctx context.Context, operation HCSOperation) {
	_, span := oc.StartSpan(ctx, "HcsCloseOperation")
	defer span.End()

	hcsCloseOperation(operation)
}

func HcsGetOperationContext(ctx context.Context, operation HCSOperation) HCSContext {
	_, span := oc.StartSpan(ctx, "HcsGetOperationContext")
	defer span.End()

	return hcsGetOperationContext(operation)
}

func HcsSetOperationContext(ctx context.Context, operation HCSOperation, callbackContext HCSContext) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsSetOperationContext")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsSetOperationContext(operation, callbackContext)
	})
}

func HcsGetComputeSystemFromOperation(ctx context.Context, operation HCSOperation) HCSSystem {
	_, span := oc.StartSpan(ctx, "HcsGetComputeSystemFromOperation")
	defer span.End()

	return hcsGetComputeSystemFromOperation(operation)
}

func HcsGetProcessFromOperation(ctx context.Context, operation HCSOperation) HCSProcess {
	_, span := oc.StartSpan(ctx, "HcsGetProcessFromOperation")
	defer span.End()

	return hcsGetProcessFromOperation(operation)
}

func HcsGetOperationType(ctx context.Context, operation HCSOperation) int32 {
	_, span := oc.StartSpan(ctx, "HcsGetOperationType")
	defer span.End()

	return hcsGetOperationType(operation)
}

func HcsGetOperationID(ctx context.Context, operation HCSOperation) uint64 {
	_, span := oc.StartSpan(ctx, "HcsGetOperationId")
	defer span.End()

	return hcsGetOperationId(operation)
}

func HcsGetOperationResult(ctx context.Context, operation HCSOperation) (resultDocument json.RawMessage, hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsGetOperationResult")
	defer span.End()
	defer func() {
		if len(resultDocument) > 0 {
			span.AddAttributes(trace.StringAttribute("resultDocument", string(resultDocument)))
		}
		oc.SetSpanStatus(span, hr)
	}()

	return resultDocument, execute(ctx, timeout.SyscallWatcher, func() error {
		var resultDocumentp *uint16
		err := hcsGetOperationResult(operation, &resultDocumentp)
		if resultDocumentp != nil {
			resultDocument = json.RawMessage(interop.ConvertAndFreeCoTaskMemString(resultDocumentp))
		}
		return err
	})
}

func HcsGetOperationResultAndProcessInfo(ctx context.Context, operation HCSOperation) (processInformation HCSProcessInformation, resultDocument string, hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsGetOperationResultAndProcessInfo")
	defer span.End()
	defer func() {
		if resultDocument != "" {
			span.AddAttributes(trace.StringAttribute("resultDocument", resultDocument))
		}
		oc.SetSpanStatus(span, hr)
	}()

	return processInformation, resultDocument, execute(ctx, timeout.SyscallWatcher, func() error {
		var resultDocumentp *uint16
		err := hcsGetOperationResultAndProcessInfo(operation, &processInformation, &resultDocumentp)
		if resultDocumentp != nil {
			resultDocument = interop.ConvertAndFreeCoTaskMemString(resultDocumentp)
		}
		return err
	})
}

func HcsAddResourceToOperation(ctx context.Context, operation HCSOperation, resourceType uint32, uri string, handle windows.Handle) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsAddResourceToOperation")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.StringAttribute("uri", uri))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsAddResourceToOperation(operation, resourceType, uri, handle)
	})
}

func HcsGetProcessorCompatibilityFromSavedState(ctx context.Context, runtimeFileName string) (processorFeaturesString string, hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsGetProcessorCompatibilityFromSavedState")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.StringAttribute("runtimeFileName", runtimeFileName))

	return processorFeaturesString, execute(ctx, timeout.SyscallWatcher, func() error {
		var processorFeaturesStringp *uint16
		err := hcsGetProcessorCompatibilityFromSavedState(runtimeFileName, &processorFeaturesStringp)
		if processorFeaturesStringp != nil {
			processorFeaturesString = interop.ConvertAndFreeCoTaskMemString(processorFeaturesStringp)
		}
		return err
	})
}

func HcsWaitForOperationResult(ctx context.Context, operation HCSOperation, timeoutMs uint32) (resultDocument string, hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsWaitForOperationResult")
	defer span.End()
	defer func() {
		if resultDocument != "" {
			span.AddAttributes(trace.StringAttribute("resultDocument", resultDocument))
		}
		oc.SetSpanStatus(span, hr)
	}()

	return resultDocument, execute(ctx, timeout.SyscallWatcher, func() error {
		var resultDocumentp *uint16
		err := hcsWaitForOperationResult(operation, timeoutMs, &resultDocumentp)
		if resultDocumentp != nil {
			resultDocument = interop.ConvertAndFreeCoTaskMemString(resultDocumentp)
		}
		return err
	})
}

func HcsWaitForOperationResultAndProcessInfo(ctx context.Context, operation HCSOperation, timeoutMs uint32) (processInformation HCSProcessInformation, resultDocument string, hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsWaitForOperationResultAndProcessInfo")
	defer span.End()
	defer func() {
		if resultDocument != "" {
			span.AddAttributes(trace.StringAttribute("resultDocument", resultDocument))
		}
		oc.SetSpanStatus(span, hr)
	}()

	return processInformation, resultDocument, execute(ctx, timeout.SyscallWatcher, func() error {
		var resultDocumentp *uint16
		err := hcsWaitForOperationResultAndProcessInfo(operation, timeoutMs, &processInformation, &resultDocumentp)
		if resultDocumentp != nil {
			resultDocument = interop.ConvertAndFreeCoTaskMemString(resultDocumentp)
		}
		return err
	})
}

func HcsSetOperationCallback(ctx context.Context, operation HCSOperation, callbackContext HCSContext, callback HCSCallback) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsSetOperationCallback")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsSetOperationCallback(operation, callbackContext, callback)
	})
}

func HcsCancelOperation(ctx context.Context, operation HCSOperation) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsCancelOperation")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsCancelOperation(operation)
	})
}

func HcsGetOperationProperties(ctx context.Context, operation HCSOperation, options string) (resultDocument string, hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsGetOperationProperties")
	defer span.End()
	defer func() {
		if resultDocument != "" {
			span.AddAttributes(trace.StringAttribute("resultDocument", resultDocument))
		}
		oc.SetSpanStatus(span, hr)
	}()
	span.AddAttributes(trace.StringAttribute("options", options))

	return resultDocument, execute(ctx, timeout.SyscallWatcher, func() error {
		var resultDocumentp *uint16
		err := hcsGetOperationProperties(operation, options, &resultDocumentp)
		if resultDocumentp != nil {
			resultDocument = interop.ConvertAndFreeCoTaskMemString(resultDocumentp)
		}
		return err
	})
}

// Compute system lifecycle

func HcsEnumerateComputeSystems(ctx context.Context, query string, operation HCSOperation) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsEnumerateComputeSystems")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.StringAttribute("query", query))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsEnumerateComputeSystems(query, operation)
	})
}

func HcsEnumerateComputeSystemsInNamespace(ctx context.Context, idNamespace string, query string, operation HCSOperation) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsEnumerateComputeSystemsInNamespace")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(
		trace.StringAttribute("idNamespace", idNamespace),
		trace.StringAttribute("query", query))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsEnumerateComputeSystemsInNamespace(idNamespace, query, operation)
	})
}

func HcsCreateComputeSystem(ctx context.Context, id string, configuration string, operation HCSOperation, securityDescriptor unsafe.Pointer) (computeSystem HCSSystem, hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsCreateComputeSystem")
	defer span.End()
	defer func() {
		if hr != errVmcomputeOperationPending { //nolint:errorlint // explicitly returned
			oc.SetSpanStatus(span, hr)
		}
	}()
	span.AddAttributes(
		trace.StringAttribute("id", id),
		trace.StringAttribute("configuration", configuration))

	return computeSystem, execute(ctx, timeout.SystemCreate, func() error {
		return hcsCreateComputeSystem(id, configuration, operation, securityDescriptor, &computeSystem)
	})
}

func HcsCreateComputeSystemInNamespace(ctx context.Context, idNamespace string, id string, configuration string, operation HCSOperation, options unsafe.Pointer) (computeSystem HCSSystem, hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsCreateComputeSystemInNamespace")
	defer span.End()
	defer func() {
		if hr != errVmcomputeOperationPending { //nolint:errorlint // explicitly returned
			oc.SetSpanStatus(span, hr)
		}
	}()
	span.AddAttributes(
		trace.StringAttribute("idNamespace", idNamespace),
		trace.StringAttribute("id", id),
		trace.StringAttribute("configuration", configuration))

	return computeSystem, execute(ctx, timeout.SystemCreate, func() error {
		return hcsCreateComputeSystemInNamespace(idNamespace, id, configuration, operation, options, &computeSystem)
	})
}

func HcsOpenComputeSystem(ctx context.Context, id string, requestedAccess uint32) (computeSystem HCSSystem, hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsOpenComputeSystem")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()

	return computeSystem, execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsOpenComputeSystem(id, requestedAccess, &computeSystem)
	})
}

func HcsOpenComputeSystemInNamespace(ctx context.Context, idNamespace string, id string, requestedAccess uint32) (computeSystem HCSSystem, hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsOpenComputeSystemInNamespace")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.StringAttribute("idNamespace", idNamespace))

	return computeSystem, execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsOpenComputeSystemInNamespace(idNamespace, id, requestedAccess, &computeSystem)
	})
}

func HcsCloseComputeSystem(ctx context.Context, computeSystem HCSSystem) {
	_, span := oc.StartSpan(ctx, "HcsCloseComputeSystem")
	defer span.End()

	hcsCloseComputeSystem(computeSystem)
}

func HcsStartComputeSystem(ctx context.Context, computeSystem HCSSystem, operation HCSOperation, options string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsStartComputeSystem")
	defer span.End()
	defer func() {
		if hr != errVmcomputeOperationPending { //nolint:errorlint // explicitly returned
			oc.SetSpanStatus(span, hr)
		}
	}()
	span.AddAttributes(trace.StringAttribute("options", options))

	return execute(ctx, timeout.SystemStart, func() error {
		return hcsStartComputeSystem(computeSystem, operation, options)
	})
}

func HcsShutDownComputeSystem(ctx context.Context, computeSystem HCSSystem, operation HCSOperation, options string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsShutDownComputeSystem")
	defer span.End()
	defer func() {
		if hr != errVmcomputeOperationPending { //nolint:errorlint // explicitly returned
			oc.SetSpanStatus(span, hr)
		}
	}()
	span.AddAttributes(trace.StringAttribute("options", options))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsShutDownComputeSystem(computeSystem, operation, options)
	})
}

func HcsTerminateComputeSystem(ctx context.Context, computeSystem HCSSystem, operation HCSOperation, options string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsTerminateComputeSystem")
	defer span.End()
	defer func() {
		if hr != errVmcomputeOperationPending { //nolint:errorlint // explicitly returned
			oc.SetSpanStatus(span, hr)
		}
	}()
	span.AddAttributes(trace.StringAttribute("options", options))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsTerminateComputeSystem(computeSystem, operation, options)
	})
}

func HcsCrashComputeSystem(ctx context.Context, computeSystem HCSSystem, operation HCSOperation, options string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsCrashComputeSystem")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.StringAttribute("options", options))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsCrashComputeSystem(computeSystem, operation, options)
	})
}

func HcsPauseComputeSystem(ctx context.Context, computeSystem HCSSystem, operation HCSOperation, options string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsPauseComputeSystem")
	defer span.End()
	defer func() {
		if hr != errVmcomputeOperationPending { //nolint:errorlint // explicitly returned
			oc.SetSpanStatus(span, hr)
		}
	}()
	span.AddAttributes(trace.StringAttribute("options", options))

	return execute(ctx, timeout.SystemPause, func() error {
		return hcsPauseComputeSystem(computeSystem, operation, options)
	})
}

func HcsResumeComputeSystem(ctx context.Context, computeSystem HCSSystem, operation HCSOperation, options string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsResumeComputeSystem")
	defer span.End()
	defer func() {
		if hr != errVmcomputeOperationPending { //nolint:errorlint // explicitly returned
			oc.SetSpanStatus(span, hr)
		}
	}()
	span.AddAttributes(trace.StringAttribute("options", options))

	return execute(ctx, timeout.SystemResume, func() error {
		return hcsResumeComputeSystem(computeSystem, operation, options)
	})
}

func HcsSaveComputeSystem(ctx context.Context, computeSystem HCSSystem, operation HCSOperation, options string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsSaveComputeSystem")
	defer span.End()
	defer func() {
		if hr != errVmcomputeOperationPending { //nolint:errorlint // explicitly returned
			oc.SetSpanStatus(span, hr)
		}
	}()

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsSaveComputeSystem(computeSystem, operation, options)
	})
}

func HcsGetComputeSystemProperties(ctx context.Context, computeSystem HCSSystem, operation HCSOperation, propertyQuery string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsGetComputeSystemProperties")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.StringAttribute("propertyQuery", propertyQuery))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsGetComputeSystemProperties(computeSystem, operation, propertyQuery)
	})
}

func HcsModifyComputeSystem(ctx context.Context, computeSystem HCSSystem, operation HCSOperation, configuration string, identity windows.Handle) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsModifyComputeSystem")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.StringAttribute("configuration", configuration))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsModifyComputeSystem(computeSystem, operation, configuration, identity)
	})
}

func HcsWaitForComputeSystemExit(ctx context.Context, computeSystem HCSSystem, timeoutMs uint32) (result string, hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsWaitForComputeSystemExit")
	defer span.End()
	defer func() {
		if result != "" {
			span.AddAttributes(trace.StringAttribute("result", result))
		}
		oc.SetSpanStatus(span, hr)
	}()

	return result, execute(ctx, timeout.SyscallWatcher, func() error {
		var resultp *uint16
		err := hcsWaitForComputeSystemExit(computeSystem, timeoutMs, &resultp)
		if resultp != nil {
			result = interop.ConvertAndFreeCoTaskMemString(resultp)
		}
		return err
	})
}

func HcsSetComputeSystemCallback(ctx context.Context, computeSystem HCSSystem, callbackOptions uint32, callbackContext HCSContext, callback HCSCallback) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsSetComputeSystemCallback")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsSetComputeSystemCallback(computeSystem, callbackOptions, callbackContext, callback)
	})
}

// Live migration

func HcsInitializeLiveMigrationOnSource(ctx context.Context, computeSystem HCSSystem, operation HCSOperation, options string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsInitializeLiveMigrationOnSource")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.StringAttribute("options", options))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsInitializeLiveMigrationOnSource(computeSystem, operation, options)
	})
}

func HcsStartLiveMigrationOnSource(ctx context.Context, computeSystem HCSSystem, operation HCSOperation, options string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsStartLiveMigrationOnSource")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.StringAttribute("options", options))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsStartLiveMigrationOnSource(computeSystem, operation, options)
	})
}

func HcsStartLiveMigrationTransfer(ctx context.Context, computeSystem HCSSystem, operation HCSOperation, options string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsStartLiveMigrationTransfer")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.StringAttribute("options", options))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsStartLiveMigrationTransfer(computeSystem, operation, options)
	})
}

func HcsFinalizeLiveMigration(ctx context.Context, computeSystem HCSSystem, operation HCSOperation, options string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsFinalizeLiveMigration")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.StringAttribute("options", options))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsFinalizeLiveMigration(computeSystem, operation, options)
	})
}

// Process lifecycle

func HcsCreateProcess(ctx context.Context, computeSystem HCSSystem, processParameters string, operation HCSOperation, securityDescriptor unsafe.Pointer) (process HCSProcess, hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsCreateProcess")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	if span.IsRecordingEvents() {
		if s, err := log.ScrubProcessParameters(processParameters); err == nil {
			span.AddAttributes(trace.StringAttribute("processParameters", s))
		}
	}

	return process, execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsCreateProcess(computeSystem, processParameters, operation, securityDescriptor, &process)
	})
}

func HcsOpenProcess(ctx context.Context, computeSystem HCSSystem, pid uint32, requestedAccess uint32) (process HCSProcess, hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsOpenProcess")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.Int64Attribute("pid", int64(pid)))

	return process, execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsOpenProcess(computeSystem, pid, requestedAccess, &process)
	})
}

func HcsCloseProcess(ctx context.Context, process HCSProcess) {
	_, span := oc.StartSpan(ctx, "HcsCloseProcess")
	defer span.End()

	hcsCloseProcess(process)
}

func HcsTerminateProcess(ctx context.Context, process HCSProcess, operation HCSOperation, options string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsTerminateProcess")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsTerminateProcess(process, operation, options)
	})
}

func HcsSignalProcess(ctx context.Context, process HCSProcess, operation HCSOperation, options string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsSignalProcess")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.StringAttribute("options", options))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsSignalProcess(process, operation, options)
	})
}

func HcsGetProcessInfo(ctx context.Context, process HCSProcess, operation HCSOperation) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsGetProcessInfo")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsGetProcessInfo(process, operation)
	})
}

func HcsGetProcessProperties(ctx context.Context, process HCSProcess, operation HCSOperation, propertyQuery string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsGetProcessProperties")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsGetProcessProperties(process, operation, propertyQuery)
	})
}

func HcsModifyProcess(ctx context.Context, process HCSProcess, operation HCSOperation, settings string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsModifyProcess")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.StringAttribute("settings", settings))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsModifyProcess(process, operation, settings)
	})
}

func HcsSetProcessCallback(ctx context.Context, process HCSProcess, callbackOptions uint32, callbackContext HCSContext, callback HCSCallback) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsSetProcessCallback")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsSetProcessCallback(process, callbackOptions, callbackContext, callback)
	})
}

func HcsWaitForProcessExit(ctx context.Context, process HCSProcess, timeoutMs uint32) (result string, hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsWaitForProcessExit")
	defer span.End()
	defer func() {
		if result != "" {
			span.AddAttributes(trace.StringAttribute("result", result))
		}
		oc.SetSpanStatus(span, hr)
	}()

	return result, execute(ctx, timeout.SyscallWatcher, func() error {
		var resultp *uint16
		err := hcsWaitForProcessExit(process, timeoutMs, &resultp)
		if resultp != nil {
			result = interop.ConvertAndFreeCoTaskMemString(resultp)
		}
		return err
	})
}

// Service

func HcsGetServiceProperties(ctx context.Context, propertyQuery string) (result string, hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsGetServiceProperties")
	defer span.End()
	defer func() {
		if result != "" {
			span.AddAttributes(trace.StringAttribute("result", result))
		}
		oc.SetSpanStatus(span, hr)
	}()
	span.AddAttributes(trace.StringAttribute("propertyQuery", propertyQuery))

	return result, execute(ctx, timeout.SyscallWatcher, func() error {
		var resultp *uint16
		err := hcsGetServiceProperties(propertyQuery, &resultp)
		if resultp != nil {
			result = interop.ConvertAndFreeCoTaskMemString(resultp)
		}
		return err
	})
}

func HcsModifyServiceSettings(ctx context.Context, settings string) (result string, hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsModifyServiceSettings")
	defer span.End()
	defer func() {
		if result != "" {
			span.AddAttributes(trace.StringAttribute("result", result))
		}
		oc.SetSpanStatus(span, hr)
	}()
	span.AddAttributes(trace.StringAttribute("settings", settings))

	return result, execute(ctx, timeout.SyscallWatcher, func() error {
		var resultp *uint16
		err := hcsModifyServiceSettings(settings, &resultp)
		if resultp != nil {
			result = interop.ConvertAndFreeCoTaskMemString(resultp)
		}
		return err
	})
}

func HcsSubmitWerReport(ctx context.Context, settings string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsSubmitWerReport")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.StringAttribute("settings", settings))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsSubmitWerReport(settings)
	})
}

// File and VM access

func HcsCreateEmptyGuestStateFile(ctx context.Context, guestStateFilePath string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsCreateEmptyGuestStateFile")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.StringAttribute("guestStateFilePath", guestStateFilePath))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsCreateEmptyGuestStateFile(guestStateFilePath)
	})
}

func HcsCreateEmptyRuntimeStateFile(ctx context.Context, runtimeStateFilePath string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsCreateEmptyRuntimeStateFile")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.StringAttribute("runtimeStateFilePath", runtimeStateFilePath))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsCreateEmptyRuntimeStateFile(runtimeStateFilePath)
	})
}

func HcsGrantVMAccess(ctx context.Context, vmID string, filePath string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsGrantVmAccess")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(
		trace.StringAttribute("vmID", vmID),
		trace.StringAttribute("filePath", filePath))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsGrantVmAccess(vmID, filePath)
	})
}

func HcsRevokeVMAccess(ctx context.Context, vmID string, filePath string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsRevokeVmAccess")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(
		trace.StringAttribute("vmID", vmID),
		trace.StringAttribute("filePath", filePath))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsRevokeVmAccess(vmID, filePath)
	})
}

func HcsGrantVMGroupAccess(ctx context.Context, filePath string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsGrantVmGroupAccess")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.StringAttribute("filePath", filePath))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsGrantVmGroupAccess(filePath)
	})
}

func HcsRevokeVMGroupAccess(ctx context.Context, filePath string) (hr error) {
	ctx, span := oc.StartSpan(ctx, "HcsRevokeVmGroupAccess")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, hr) }()
	span.AddAttributes(trace.StringAttribute("filePath", filePath))

	return execute(ctx, timeout.SyscallWatcher, func() error {
		return hcsRevokeVmGroupAccess(filePath)
	})
}
