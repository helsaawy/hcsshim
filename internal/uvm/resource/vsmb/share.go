//go:build windows

package vsmb

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"

	"github.com/Microsoft/hcsshim/internal/hcs"
	hcsschema "github.com/Microsoft/hcsshim/internal/hcs/schema2"
	"github.com/Microsoft/hcsshim/internal/uvm/resource"
)

const currentSerialVersionID uint32 = 1

// Share contains the host path for a vSMB file shares.
//
// Do not modify unless holding parent controller lock, m.mu.
type Share struct {
	// Manager for this share.
	//
	// * Do not set to nil. *
	// Even after closing, still need to access m.mu to check fields.
	m *Manager

	hostPath string
	refCount uint32
	name     string
	// Files within HostPath that the vSMB share is making available to the uVM.
	// This is only non-nil iff `vsmb.options.SingleFileMapping && vsmb.options.RestrictFileAccess`.
	//
	// Append only, unless deleting list.
	allowedFiles []string
	guestPath    string
	options      hcsschema.VirtualSmbShareOptions

	serialVersionID uint32
}

var _ resource.Cloneable = &Share{}

// Release frees the resources of the corresponding vSMB mount.
func (r *Share) Release(ctx context.Context) error {
	if err := r.m.Remove(ctx, r); err != nil {
		return fmt.Errorf("failed to release vSMB share: %w", err)
	}
	return nil
}

// close closes a vSMB share.
// It should only be called after the manager has removed it from its host
//
// caller should hold the manager's lock.
func (r *Share) close(context.Context) error {
	if r.m == nil {
		return fmt.Errorf("vSMB share already closed: %w", hcs.ErrAlreadyClosed)
	}
	r.hostPath = ""
	r.name = ""
	r.guestPath = ""
	r.allowedFiles = nil
	r.options = hcsschema.VirtualSmbShareOptions{}
	return nil
}

func (r *Share) isDirShare() bool {
	// allowedFiles != nil iff (vsmb.options.SingleFileMapping && vsmb.options.RestrictFileAccess)
	return !(r.options.RestrictFileAccess && r.options.SingleFileMapping)
}

// GobEncode serializes the Share struct.
func (r *Share) GobEncode() ([]byte, error) {
	// GobEncode is called in "internal/clone".SaveTemplateConfig, where the uVM/Managers are
	// not involved at all.
	// RLock the Manager to prevent modifying the resource while encoding.
	r.m.mu.RLock()
	defer r.m.mu.RUnlock()

	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	errMsgFmt := "failed to encode Share: %s"
	// encode only the fields that can be safely deserialized.
	// Always use vsmbCurrentSerialVersionID as vsmb.serialVersionID might not have
	// been initialized.
	if err := encoder.Encode(currentSerialVersionID); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(r.hostPath); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(r.name); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(r.allowedFiles); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(r.guestPath); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(r.options); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	return buf.Bytes(), nil
}

// GobDecode deserializes the Share struct into the struct on which this is called
// (i.e the vsmb pointer).
func (r *Share) GobDecode(data []byte) error {
	// resource does not have a valid Manager/uVM to lock when decoding

	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)
	errMsgFmt := "failed to decode Share: %s"
	// fields should be decoded in the same order in which they were encoded.
	// And verify the serialVersionID first
	if err := decoder.Decode(&r.serialVersionID); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if r.serialVersionID != currentSerialVersionID {
		return fmt.Errorf(
			"serialized version of Share %d doesn't match with the current version %d",
			r.serialVersionID,
			currentSerialVersionID)
	}
	if err := decoder.Decode(&r.hostPath); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if err := decoder.Decode(&r.name); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if err := decoder.Decode(&r.allowedFiles); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if err := decoder.Decode(&r.guestPath); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if err := decoder.Decode(&r.options); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	return nil
}

// must hold r.m.mu before calling.
func (r *Share) shareKey() string {
	return shareKey(r.hostPath, r.options.ReadOnly)
}

func (*Share) Type() resource.Type { return resource.VSMB }

func (r *Share) SerialVersionID() uint32 { return r.serialVersionID }

// shareKey returns a string key which encapsulates the information that is used to
// look up an existing VSMB share. If a share is being added, but there is an existing
// share with the same key, the existing share will be used instead (and its ref count
// incremented).
func shareKey(hostPath string, readOnly bool) string {
	return fmt.Sprintf("%v-%v", hostPath, readOnly)
}
