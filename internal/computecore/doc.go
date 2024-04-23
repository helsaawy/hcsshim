// This package provides bindings and utility functions for dealing with ComputeCore.dll Win32 APIs.
//
// The HCS APIs allow using the operation as a future (and waiting or polling via
// [HcsWaitForOperationResult] or [HcsGetOperationResult], respectively), or setting an operation
// callback.
// However:
//
//  1. Futures do not match Go's async model (which instead relies on go routines).
//  2. An operation callback will error if the compute system has an event callback, which
//     would prevent users from relying on the latter to be notified of compute systems events.
//
// For that reason, the APIs are called synchronously, and can be used asynchronously using a goroutine.
//
// See HCS [operation samples] for more information.
//
// [HcsWaitForOperationResult]: https://learn.microsoft.com/en-us/virtualization/api/hcs/reference/hcswaitforoperationresult
// [HcsGetOperationResult]: https://learn.microsoft.com/en-us/virtualization/api/hcs/reference/hcsgetoperationresult
// [operation samples]: https://learn.microsoft.com/en-us/virtualization/api/hcs/reference/operationcompletionsample
package computecore
