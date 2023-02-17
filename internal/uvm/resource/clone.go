package resource

import (
	"context"

	hcsschema "github.com/Microsoft/hcsshim/internal/hcs/schema2"
)

// Cloneable is a generic interface for cloning a specific resource. Not all resources can
// be cloned and so all resources might not implement this interface. This interface is
// mainly used during late cloning process to clone the resources associated with the UVM
// and the container. For some resources (like scratch VHDs of the UVM & container)
// cloning means actually creating a copy of that resource while for some resources it
// simply means adding that resource to the cloned VM without copying (like VSMB shares).
// The Clone function of that resource will deal with these details.
type Cloneable interface {
	// A resource that supports cloning should also support serialization and
	// deserialization operations. This is because during resource cloning a resource
	// is usually serialized in one process and then deserialized and cloned in some
	// other process. Care should be taken while serializing a resource to not include
	// any state that will not be valid during the deserialization step. By default
	// gob encoding is used to serialize and deserialize resources but a resource can
	// implement `gob.GobEncoder` & `gob.GobDecoder` interfaces to provide its own
	// serialization and deserialization functions.

	Resource

	// A SerialVersionID is an identifier used to recognize a unique version of a
	// resource. Every time the definition of the resource struct changes this ID is
	// bumped up.  This ID is used to ensure that we serialize and deserialize the
	// same version of a resource.
	GetSerialVersionID() uint32

	// Clone function creates a clone of the resource on the Host (i.e adds the
	// cloned resource to the uVM)
	// `cd` parameter can be used to pass any other data that is required during the
	// cloning process of that resource (for example, when cloning SCSI Mounts we
	// might need scratchFolder).
	// Clone function should be called on a valid struct (Mostly on the struct which
	// is deserialized, and so Clone function should only depend on the fields that
	// are exported in the struct).
	// The implementation of the clone function should avoid reading any data from the
	// `vm` struct, it can add new fields to the vm struct but since the vm struct
	// isn't fully ready at this point it shouldn't be used to read any data.
	Clone(ctx context.Context, host Host, cd *CloneData) error
}

// A struct to keep all the information that might be required during cloning process of
// a resource.
type CloneData struct {
	// Doc spec for the clone
	Doc *hcsschema.ComputeSystem
	// ScratchFolder of the clone
	ScratchFolder string
	// ID is the uVM ID of the clone
	UVMID string
}
