package resource

import (
	"context"
	"errors"

	hcsschema "github.com/Microsoft/hcsshim/internal/hcs/schema2"
)

var (
	ErrInvalidManagerState  = errors.New("invalid manager state")
	ErrInvalidResourceState = errors.New("invalid resource state")

	ErrNotSupported = errors.New("not supported")

	ErrNoAvailableLocation = errors.New("no available location")
	ErrNotAttached         = errors.New("not attached")
	ErrAlreadyAttached     = errors.New("already attached")
)

// Type refers to the type of a resource on a utility VM.
type Type uint8

const (
	// VPMem is virtual persistent memory (vPMEM) devices.
	VPMem = Type(iota)
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

// Host is the compute system (eg, uVM) that is hosting the individual resources that
// the [Manager] is managing.
type Host interface {
	// type Host[R Resource] interface {
	// Manager() (Manager[R], error)

	// ID will return a string identifier for the utility VM.
	ID() string

	Manager(Type) (Manager[Resource], error)

	// Modify modifies the utility VM.
	Modify(context.Context, *hcsschema.ModifySettingRequest) error

	// DevicesPhysicallyBacked describes if additional devices added to the UVM should be physically backed.
	DevicesPhysicallyBacked() bool

	// DisallowWritableFileShares describes if writable file shares are allowed.
	DisallowWritableFileShares() bool
}

// Manager tracks and manages resources on a [ResourceHost].
type Manager[R Resource] interface {
	// ResourceType returns the type of resource that is being managed.
	ResourceType() Type

	Host() (Host, error)

	// Remove removes a particular resource, if it exists.
	Remove(context.Context, R) error

	// List returns all the currently known resources.
	List(context.Context) ([]R, error)

	// Close attempts to close all resources, returning any errors that arose.
	Close(context.Context) error
}

type Resource interface {
	Type() Type
}
