package vm

import (
	"context"
	"errors"
	"net"

	"github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/stats"
)

var (
	ErrNotSupported       = errors.New("virtstack does not support the operation")
	ErrAlreadySet         = errors.New("field has already been set")
	ErrUnsupportedGuestOS = errors.New("virtstack does not support the guest operating system")
	ErrUnknownGuestOS     = errors.New("unknown guest operating system supplied")
)

const (
	HCS      = "hcs"
	RemoteVM = "remotevm"
)

// UVM is an abstraction around a lightweight utility virtual machine.
// It houses core lifecycle methods such as Create, Start, and Stop, as well as several
// optional, nested interfaces that can be used to determine what the virtual machine
// supports and to configure these resources.
type UVM interface {
	// GuestOS returns the type of guest Operating System being run.
	GuestOS() GuestOS

	// ID will return a string identifier for the utility VM.
	ID() string

	// Start will power on the utility VM and put it into a running state. This will boot the guest OS and start all of the
	// devices configured on the machine.
	Start(ctx context.Context) error

	// Stop will shutdown the utility VM and place it into a terminated state.
	Stop(ctx context.Context) error

	// Pause will place the utility VM into a paused state. The guest OS will be halted and any devices will have be in a
	// a suspended state. Save can be used to snapshot the current state of the virtual machine, and Resume can be used to
	// place the virtual machine back into a running state.
	Pause(ctx context.Context) error

	// Resume will put a previously paused utility VM back into a running state. The guest OS will resume operation from the point
	// in time it was paused and all devices should be un-suspended.
	Resume(ctx context.Context) error

	// Save will snapshot the state of the utility VM at the point in time when the VM was paused.
	Save(ctx context.Context) error

	// Wait synchronously waits for the utility VM to shutdown or terminate. A call to stop will trigger this
	// to unblock.
	Wait() error

	// Stats returns statistics about the utility VM. This includes things like assigned memory, available memory,
	// processor runtime etc.
	Stats(ctx context.Context) (*stats.VirtualMachineStatistics, error)

	// Supported returns if the virtstack supports a given operation on a resource.
	Supported(resource Resource, operation ResourceOperation) bool

	// ExitError will return any error if the utility VM exited unexpectedly, or if the utility VM experienced an
	// error after Wait returned, it will return the wait error.
	ExitError() error

	// DevicesPhysicallyBacked describes if additional devices added to the UVM should be physically backed.
	DevicesPhysicallyBacked() bool

	// DisallowWritableFileShares describes if writable file shares are allowed.
	DisallowWritableFileShares() bool
}

// Resource refers to the type of a resource on a utility VM.
type Resource uint8

const (
	// VPMem is virtual persistent memory (vPMEM) devices.
	VPMem = Resource(iota)
	SCSI
	Network
	// VSMB is a virtual SMB file share.
	VSMB
	PCI
	// Plan9 is a Plan9 SMB file share.
	Plan9
	Memory
	Processor
	CPUGroup
)

// Operation refers to the type of operation to perform on a given resource.
type ResourceOperation uint8

const (
	Add ResourceOperation = iota
	Remove
	Update
)

// GuestOS signifies the guest operating system that a utility VM will be running.
type GuestOS string

const (
	Windows GuestOS = "windows"
	Linux   GuestOS = "linux"
)

// SCSIDiskType refers to the disk type of the SCSI device. This is either a vhd, vhdx, or a physical disk.
type SCSIDiskType uint8

const (
	SCSIDiskTypeVHD1 SCSIDiskType = iota
	SCSIDiskTypeVHDX
	SCSIDiskTypePassThrough
)

// SCSIManager manages adding and removing SCSI devices for a utility VM.
type SCSIManager interface {
	// AddSCSIController adds a SCSI controller to the utility VM configuration document.
	AddSCSIController(id uint32) error
	// AddSCSIDisk adds a SCSI disk to the configuration document if in a precreated state, or hot adds a
	// SCSI disk to the utility VM if the VM is running.
	AddSCSIDisk(ctx context.Context, controller uint32, lun uint32, path string, typ SCSIDiskType, readOnly bool) error
	// RemoveSCSIDisk removes a SCSI disk from a utility VM.
	RemoveSCSIDisk(ctx context.Context, controller uint32, lun uint32, path string) error
}

// VPMemImageFormat refers to the image type of the vPMEM block device. This is either a vhd or vhdx.
type VPMemImageFormat uint8

const (
	VPMemImageFormatVHD1 VPMemImageFormat = iota
	VPMemImageFormatVHDX
)

// VPMemManager manages adding and removing virtual persistent memory (vPMEM) devices for a utility VM.
type VPMemManager interface {
	// AddVPMemController adds a new virtual PMEM controller to the utility VM.
	// `maximumDevices` specifies how many vPMEM devices will be present in the guest.
	// `maximumSizeBytes` specifies the maximum size allowed for a vPMEM device.
	AddVPMemController(maximumDevices uint32, maximumSizeBytes uint64) error
	// AddVPMemDevice adds a virtual pmem device to the utility VM.
	AddVPMemDevice(ctx context.Context, id uint32, path string, readOnly bool, imageFormat VPMemImageFormat) error
	// RemoveVPMemDevice removes a virtual pmem device from the utility VM.
	RemoveVPMemDevice(ctx context.Context, id uint32, path string) error
}

// NetworkManager manages adding and removing network adapters for a utility VM.
type NetworkManager interface {
	// AddNIC adds a network adapter to the utility VM. `nicID` should be a string representation of a
	// Windows GUID.
	AddNIC(ctx context.Context, nicID string, endpointID string, macAddr string) error
	// RemoveNIC removes a network adapter from the utility VM. `nicID` should be a string representation of a
	// Windows GUID.
	RemoveNIC(ctx context.Context, nicID string, endpointID string, macAddr string) error
}

// PCIManager manages assigning pci devices to a utility VM.
// This is Windows specific at the moment.
type PCIManager interface {
	// AddDevice adds the pci device identified by `instanceID` to the utility VM.
	//
	// https://docs.microsoft.com/en-us/windows-hardware/drivers/install/instance-ids
	AddDevice(ctx context.Context, instanceID string, vmbusGUID string) error
	// RemoveDevice removes the pci device identified by `instanceID` from the utility VM.
	RemoveDevice(ctx context.Context, instanceID string, vmbusGUID string) error
}

// VMSocketType refers to which hypervisor socket transport type to use.
type VMSocketType uint8

const (
	HvSocket VMSocketType = iota
	VSock
)

// VMSocketManager manages configuration for a hypervisor socket transport. This includes sockets such as
// HvSocket and Vsock.
type VMSocketManager interface {
	// VMSocketListen will create the requested vmsocket type and listen on the address specified by `connID`.
	// For HvSocket the type expected is a GUID, for Vsock it's a port of type uint32.
	VMSocketListen(ctx context.Context, socketType VMSocketType, connID interface{}) (net.Listener, error)
}

// VSMBOptions
type VSMBOptions struct {
	ReadOnly            bool
	CacheIo             bool
	NoDirectMap         bool
	PseudoOplocks       bool
	ShareRead           bool
	TakeBackupPrivilege bool
	NoOplocks           bool
	PseudoDirnotify     bool
}

// VSMBManager manages adding virtual SMB (vSMB) shares to a utility VM.
type VSMBManager interface {
	// AddVSMB adds a virtual SMB share to a running utility VM.
	AddVSMB(ctx context.Context, hostPath string, name string, allowedFiles []string, options *VSMBOptions) error
	// RemoveVSMB removes a virtual smb share from a running utility VM.
	RemoveVSMB(ctx context.Context, name string) error
}

// Plan9Manager manages adding plan 9 shares to a utility VM.
type Plan9Manager interface {
	// AddPlan9 adds a plan 9 share to a running utility VM.
	AddPlan9(ctx context.Context, path, name string, port int32, flags int32, allowed []string) error
	// RemovePlan9 removes a plan 9 share from a running utility VM.
	RemovePlan9(ctx context.Context, name string, port int32) error
}
