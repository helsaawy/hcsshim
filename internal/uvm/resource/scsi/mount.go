//go:build windows

package scsi

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"

	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/uvm/resource"
	"github.com/sirupsen/logrus"
)

const currentSerialVersionID = 2

// Mount struct representing a SCSI mount point and the uVM it belongs to.
//
// Do not modify unless holding parent controller lock, m.mu.
type Mount struct {
	// manager for this share
	//
	// * Do not set to nil. *
	// Even after closing, still need to access m.mu to check fields.
	m *Manager

	// path is the host path to the vhd that is mounted.
	hostPath string
	// path for the uvm
	guestPath string
	// SCSI controller
	controller int
	// SCSI logical unit number
	lun int32
	// While most VHDs attached to SCSI are scratch spaces, in the case of LCOW
	// when the size is over the size possible to attach to PMEM, we use SCSI for
	// read-only layers. As RO layers are shared, we perform ref-counting.
	isLayer  bool
	refCount uint32
	// specifies if this is an encrypted VHD
	encrypted bool
	// specifies if this is a readonly layer
	readOnly bool
	// "VirtualDisk" or "PassThru" or "ExtensibleVirtualDisk" disk attachment type.
	attachmentType AttachmentType
	// If attachmentType is "ExtensibleVirtualDisk" then extensibleVirtualDiskType should
	// specify the type of it (for e.g "space" for storage spaces). Otherwise this should be
	// empty.
	extensibleVirtualDiskType string

	// A channel to wait on while mount of this SCSI disk is in progress.
	waitCh chan struct{}
	// The error field that is set if the mounting of this disk fails. Any other waiters on waitCh
	// can use this waitErr after the channel is closed.
	waitErr error

	// serialization ID
	// Make sure that serialVersionID is always the last field and its value is
	// incremented every time this structure is updated
	serialVersionID uint32
}

var _ resource.Cloneable = &Mount{}

// NewMount creates a new [Mount] struct, but does not add it to the manager's internal state.
func (m *Manager) NewMount(
	hostPath string,
	guestPath string,
	attachmentType AttachmentType,
	evdType string,
	refCount uint32,
	controller int,
	lun int32,
	readOnly bool,
	encrypted bool,
) *Mount {
	return &Mount{
		m:                         m,
		hostPath:                  hostPath,
		guestPath:                 guestPath,
		refCount:                  refCount,
		controller:                controller,
		lun:                       int32(lun),
		encrypted:                 encrypted,
		readOnly:                  readOnly,
		attachmentType:            attachmentType,
		extensibleVirtualDiskType: evdType,
		serialVersionID:           currentSerialVersionID,
		waitCh:                    make(chan struct{}),
	}
}

// Release frees the resources of the corresponding SCSI Mount
func (r *Mount) Release(ctx context.Context) error {
	if err := r.m.Remove(ctx, r); err != nil {
		return fmt.Errorf("failed to remove SCSI device: %s", err)
	}
	return nil
}

// close closes a SCSI mount.
// It should only be called after the manager has removed it from its host
//
// caller should hold the manager's lock.
func (r *Mount) close(context.Context) error {
	if r.m == nil {
		return fmt.Errorf("vSMB share already closed: %w", hcs.ErrAlreadyClosed)
	}

	r.hostPath = ""
	r.guestPath = ""
	r.controller = 0
	r.lun = 0
	r.waitCh = nil
	r.waitErr = nil
	return nil
}

// GobEncode serializes the Mount struct
func (r *Mount) GobEncode() ([]byte, error) {
	// GobEncode is called in "internal/clone".SaveTemplateConfig, where the uVM/Managers are
	// not involved at all.
	// RLock the Manager to prevent modifying the resource while encoding.

	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	errMsgFmt := "failed to encode Mount: %s"
	// encode only the fields that can be safely deserialized.
	if err := encoder.Encode(r.serialVersionID); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(r.hostPath); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(r.guestPath); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(r.controller); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(r.lun); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(r.readOnly); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(r.attachmentType); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(r.extensibleVirtualDiskType); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	return buf.Bytes(), nil
}

// GobDecode deserializes the Mount struct into the struct on which this is called
// (i.e the sm pointer)
func (r *Mount) GobDecode(data []byte) error {
	// resource does not have a valid Manager/uVM to lock when decoding
	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)
	errMsgFmt := "failed to decode Mount: %s"
	// fields should be decoded in the same order in which they were encoded.
	if err := decoder.Decode(&r.serialVersionID); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if r.serialVersionID != currentSerialVersionID {
		return fmt.Errorf("serialized version of Mount: %d doesn't match with the current version: %d", r.serialVersionID, currentSerialVersionID)
	}
	if err := decoder.Decode(&r.hostPath); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if err := decoder.Decode(&r.guestPath); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if err := decoder.Decode(&r.controller); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if err := decoder.Decode(&r.lun); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if err := decoder.Decode(&r.readOnly); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if err := decoder.Decode(&r.attachmentType); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if err := decoder.Decode(&r.extensibleVirtualDiskType); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	return nil
}

func (r *Mount) GuestPath() string {
	r.m.mu.RLock()
	defer r.m.mu.RUnlock()
	return r.guestPath
}

func (r *Mount) Controller() int {
	r.m.mu.RLock()
	defer r.m.mu.RUnlock()
	return r.controller
}

func (r *Mount) LUN() int32 {
	r.m.mu.RLock()
	defer r.m.mu.RUnlock()
	return r.lun
}

// RefCount returns the current refcount for the SCSI mount.
func (r *Mount) RefCount() uint32 {
	r.m.mu.RLock()
	defer r.m.mu.RUnlock()
	return r.refCount
}

// must hold r.m.mu before calling.
func (r *Mount) entry(ctx context.Context) *logrus.Entry {
	return log.G(ctx).WithFields(logrus.Fields{
		"HostPath":                  r.hostPath,
		"UVMPath":                   r.guestPath,
		"isLayer":                   r.isLayer,
		"refCount":                  r.refCount,
		"Controller":                r.controller,
		"LUN":                       r.lun,
		"ExtensibleVirtualDiskType": r.extensibleVirtualDiskType,
		"SerialVersionID":           r.serialVersionID,
	})
}

func (*Mount) Type() resource.Type { return resource.SCSI }

func (r *Mount) SerialVersionID() uint32 { return r.serialVersionID }
