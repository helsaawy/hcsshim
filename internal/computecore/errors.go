//go:build windows

package computecore

import (
	"golang.org/x/sys/windows"
)

// computecore specific HResults

// HCSOperation
const (
	// The operation has not started.
	ErrOperationNotStarted = windows.Errno(0x80370115)
	// The operation is already running.
	ErrOperationAlreadyStarted = windows.Errno(0x80370116)
	// The operation is still running.
	ErrOperationPending = windows.Errno(0x80370117)
	// The operation did not complete in time.
	ErrOperationTimeout = windows.Errno(0x80370118)
	// An event callback has already been registered on this handle.
	ErrOperationSystemCallbackAlreadySet = windows.Errno(0x80370119)
	// Not enough memory available to return the result of the operation.
	ErrOperationResultAllocationFailed = windows.Errno(0x8037011A)
	// The operation has already been cancelled.
	ErrOperationAlreadyCancelled = windows.Errno(0x80370121)
)
