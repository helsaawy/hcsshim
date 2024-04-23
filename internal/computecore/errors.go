//go:build windows

package computecore

import "golang.org/x/sys/windows"

const hcsHResultPrefix = 0x80370000

// HCS specific error codes.
//
// See [documentation] for more info.
//
// [documentation]: https://learn.microsoft.com/en-us/virtualization/api/hcs/reference/hcshresult
//
//nolint:stylecheck // ST1003: ALL_CAPS
const (
	// The virtual machine or container exited unexpectedly while starting.
	HCS_E_TERMINATED_DURING_START = windows.Errno(hcsHResultPrefix + 0x0100)

	// The container operating system does not match the host operating system.
	HCS_E_IMAGE_MISMATCH = windows.Errno(hcsHResultPrefix + 0x0101)

	// The virtual machine could not be started because a required feature is not installed.
	HCS_E_HYPERV_NOT_INSTALLED = windows.Errno(hcsHResultPrefix + 0x0102)

	// The requested virtual machine or container operation is not valid in the current state.
	HCS_E_INVALID_STATE = windows.Errno(hcsHResultPrefix + 0x0105)

	// The virtual machine or container exited unexpectedly.
	HCS_E_UNEXPECTED_EXIT = windows.Errno(hcsHResultPrefix + 0x0106)

	// The virtual machine or container was forcefully exited.
	HCS_E_TERMINATED = windows.Errno(hcsHResultPrefix + 0x0107)

	// A connection could not be established with the container or virtual machine.
	HCS_E_CONNECT_FAILED = windows.Errno(hcsHResultPrefix + 0x0108)

	// The operation timed out because a response was not received from the virtual machine or container.
	HCS_E_CONNECTION_TIMEOUT = windows.Errno(hcsHResultPrefix + 0x0109)

	// The connection with the virtual machine or container was closed.
	HCS_E_CONNECTION_CLOSED = windows.Errno(hcsHResultPrefix + 0x010A)

	// An unknown internal message was received by the virtual machine or container.
	HCS_E_UNKNOWN_MESSAGE = windows.Errno(hcsHResultPrefix + 0x010B)

	// The virtual machine or container does not support an available version of the communication protocol with the host.
	HCS_E_UNSUPPORTED_PROTOCOL_VERSION = windows.Errno(hcsHResultPrefix + 0x010C)

	// The virtual machine or container JSON document is invalid.
	HCS_E_INVALID_JSON = windows.Errno(hcsHResultPrefix + 0x010D)

	// A virtual machine or container with the specified identifier does not exist.
	HCS_E_SYSTEM_NOT_FOUND = windows.Errno(hcsHResultPrefix + 0x010E)

	// A virtual machine or container with the specified identifier already exists.
	HCS_E_SYSTEM_ALREADY_EXISTS = windows.Errno(hcsHResultPrefix + 0x010F)

	// The virtual machine or container with the specified identifier is not running.
	HCS_E_SYSTEM_ALREADY_STOPPED = windows.Errno(hcsHResultPrefix + 0x0110)

	// A communication protocol error has occurred between the virtual machine or container and the host.
	HCS_E_PROTOCOL_ERROR = windows.Errno(hcsHResultPrefix + 0x0111)

	// The container image contains a layer with an unrecognized format.
	HCS_E_INVALID_LAYER = windows.Errno(hcsHResultPrefix + 0x0112)

	// To use this container image, you must join the Windows Insider Program.
	// Please see https://go.microsoft.com/fwlink/?linkid=850659 for more information.
	HCS_E_WINDOWS_INSIDER_REQUIRED = windows.Errno(hcsHResultPrefix + 0x0113)

	// The operation could not be started because a required feature is not installed.
	HCS_E_SERVICE_NOT_AVAILABLE = windows.Errno(hcsHResultPrefix + 0x0114)

	// The operation has not started.
	HCS_E_OPERATION_NOT_STARTED = windows.Errno(hcsHResultPrefix + 0x0115)

	// The operation is already running.
	HCS_E_OPERATION_ALREADY_STARTED = windows.Errno(hcsHResultPrefix + 0x0116)

	// The operation is still running.
	HCS_E_OPERATION_PENDING = windows.Errno(hcsHResultPrefix + 0x0117)

	// The operation did not complete in time.
	HCS_E_OPERATION_TIMEOUT = windows.Errno(hcsHResultPrefix + 0x0118)

	// An event callback has already been registered on this handle.
	HCS_E_OPERATION_SYSTEM_CALLBACK_ALREADY_SET = windows.Errno(hcsHResultPrefix + 0x0119)

	// Not enough memory available to return the result of the operation.
	HCS_E_OPERATION_RESULT_ALLOCATION_FAILED = windows.Errno(hcsHResultPrefix + 0x011A)

	// Insufficient privileges.
	// Only administrators or users that are members of the Hyper-V Administrators user group are permitted to access virtual machines or containers.
	// To add yourself to the Hyper-V Administrators user group, please see https://aka.ms/hcsadmin for more information.
	HCS_E_ACCESS_DENIED = windows.Errno(hcsHResultPrefix + 0x011B)

	// The virtual machine or container reported a critical error and was stopped or restarted.
	HCS_E_GUEST_CRITICAL_ERROR = windows.Errno(hcsHResultPrefix + 0x011C)

	// The process information is not available.
	HCS_E_PROCESS_INFO_NOT_AVAILABLE = windows.Errno(hcsHResultPrefix + 0x011D)

	// The host compute system service has disconnected unexpectedly.
	HCS_E_SERVICE_DISCONNECT = windows.Errno(hcsHResultPrefix + 0x011E)

	// The process has already exited.
	HCS_E_PROCESS_ALREADY_STOPPED = windows.Errno(hcsHResultPrefix + 0x011F)

	// The virtual machine or container is not configured to perform the operation.
	HCS_E_SYSTEM_NOT_CONFIGURED_FOR_OPERATION = windows.Errno(hcsHResultPrefix + 0x0120)

	// The operation has already been cancelled.
	HCS_E_OPERATION_ALREADY_CANCELLED = windows.Errno(hcsHResultPrefix + 0x0121)
)
