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

// Because of weird generics rules, we cannot have an non-generic interface with a generic function
// (ie, type constraints must be at the interface level).
// Additionally, we cannot use a non-concrete generic interfaces:
//
//	GetManager() Manager
//
// Finally, Go does not allow generic methods:
//
//	(*H) GetManager[R Resource]() Manager[R]
//
// This requires either polluting all the interfaces with type-constraints, or doing manual,
// run-time casting. We chose the later case, and push the choosing of the appropriate
// manager for a resource into the Host methods that require them (ie, (vm).Clone).

// Host is the compute system (eg, uVM) that is hosting the individual resources that
// the a [Manager] is managing.
//
// It is the resource host, but will usually be guest compute system runing on a host (root) operating system.
type Host interface {
	// type Host[R Resource] interface {
	// Manager() (Manager[R], error)

	// ID will return a string identifier for the utility VM.
	ID() string

	// OS returns the operating system of Host.
	OS() string

	// Modify modifies the utility VM.
	Modify(context.Context, *hcsschema.ModifySettingRequest) error

	// DevicesPhysicallyBacked describes if additional devices added to the UVM should be physically backed.
	DevicesPhysicallyBacked() bool
}

// Manager tracks and manages resources on a [Host].
type Manager[R Resource] interface {
	Host() (Host, error)

	// Remove removes a particular resource, if it exists.
	Remove(context.Context, R) error

	// List returns all the currently known resources.
	List(context.Context) ([]R, error)

	// Close attempts to close all resources, returning any errors that arose.
	Close(context.Context) error
}

//! Caveat emptor:
//! ideally Resources would first hold a lock, then check that they (r Resource)
//! and their manager (r.m *Mannager) are non-nil, but that presents a race condition,
//! since attempting to hold r.m.mu would panic regardless
//!
//! Callers should be careful when operating on resources directly

type Resource interface {
	// Type returns the resource [Type] of this Resource.
	Type() Type // mostly just to have an explicit method for making a struct a [Resource]

	// Release frees the underlying resources.
	Release(context.Context) error // resources.ResourceCloser, cant import here cause of cycles
}

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
