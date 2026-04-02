//go:build windows

package computecore

//	typedef struct HCS_EVENT
//	{
//	    HCS_EVENT_TYPE Type;
//	    PCWSTR EventData;
//	    HCS_OPERATION Operation;
//	} HCS_EVENT;

// HCSEvent provides information about an event that occurred on a compute system or process.
//
// See [documentation] for more info.
//
// [documentation]: https://learn.microsoft.com/en-us/virtualization/api/hcs/reference/hcs_event
type HCSEvent struct {
	Type HCSEventType
	// EventData provides additional data for the event as a JSON document.
	//
	// The type of JSON document depends on the event type:
	//
	// - [EventSystemExited]:			[schema2.SystemExitStatus]
	// - [EventSystemCrashInitiated]:	[schema2.CrashReport]
	// - [EventSystemCrashReport]:		[schema2.CrashReport]
	// - [EventProcessExited]:			[schema2.ProcessStatus]
	EventData *uint16
	// Handle to a completed operation, if Type is eventOperationCallback.
	// This is only possible when HcsSetComputeSystemCallback has specified event option HcsEventOptionEnableOperationCallbacks.
	Operation HCSOperation
}

//go:generate go run golang.org/x/tools/cmd/stringer -type=HCSEventType -trimprefix=Event event.go

// HCSEventType indicates the event type for callbacks registered by hcsSetComputeSystemCallback or hcsSetProcessCallback.
//
// See [documentation] for more info.
//
// [documentation]: https://learn.microsoft.com/en-us/virtualization/api/hcs/reference/hcs_event_type
type HCSEventType int32

const (
	EventInvalid                           = HCSEventType(0x00000000)
	EventSystemExited                      = HCSEventType(0x00000001)
	EventSystemCrashInitiated              = HCSEventType(0x00000002)
	EventSystemCrashReport                 = HCSEventType(0x00000003)
	EventSystemRdpEnhancedModeStateChanged = HCSEventType(0x00000004)
	EventSystemSiloJobCreated              = HCSEventType(0x00000005)
	EventSystemGuestConnectionClosed       = HCSEventType(0x00000006)
	EventProcessExited                     = HCSEventType(0x00010000)
	EventOperationCallback                 = HCSEventType(0x01000000)
	EventServiceDisconnect                 = HCSEventType(0x02000000)
)

//go:generate go run golang.org/x/tools/cmd/stringer -type=HCSEventOptions -trimprefix=EventOption event.go

// HCSEventOptions defines the options for an event callback registration, used in HcsSetComputeSystemCallback and HcsSetProcessCallback.
//
// See [documentation] for more info.
//
// [documentation]: https://learn.microsoft.com/en-us/virtualization/api/hcs/reference/hcs_event_options
type HCSEventOptions int32

const (
	EventOptionNone                     = HCSEventOptions(0x00000000)
	EventOptionEnableOperationCallbacks = HCSEventOptions(0x00000001)
)
