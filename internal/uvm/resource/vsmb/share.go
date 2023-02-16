//go:build windows

package vsmb

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"

	"github.com/Microsoft/hcsshim/internal/hcs"
	hcsschema "github.com/Microsoft/hcsshim/internal/hcs/schema2"
	"github.com/Microsoft/hcsshim/internal/vm"
)

// Share contains the host path for a vSMB mount.
//
// Do not modify unless holding parent controller lock, m.mu.
type Share struct {
	// controller the resource belongs to
	m *Manager

	HostPath string
	refCount uint32
	name     string
	// files within HostPath that the vSMB share is making available to the uVM.
	// This is only non-nil iff `vsmb.options.SingleFileMapping && vsmb.options.RestrictFileAccess`.
	//
	// Append only, unless deleting list.
	allowedFiles    []string
	guestPath       string
	options         hcsschema.VirtualSmbShareOptions
	serialVersionID uint32
}

var _ vm.Cloneable = &Share{}

// Release frees the resources of the corresponding vSMB mount.
func (s *Share) Release(ctx context.Context) error {
	if err := s.m.RemoveShare(ctx, s); err != nil {
		return fmt.Errorf("failed to release vSMB share: %w", err)
	}
	return nil
}

// close closes a vSMB share.
// It is unsynchronized; the caller should hold the manager's lock.
func (s *Share) close(context.Context) error {
	if s.m == nil {
		return fmt.Errorf("vSMB share already closed: %w", hcs.ErrAlreadyClosed)
	}
	s.HostPath = ""
	s.allowedFiles = nil
	s.options = hcsschema.VirtualSmbShareOptions{}
	s.m = nil
	return nil
}

func (s *Share) isDirShare() bool {
	// allowedFiles != nil iff (vsmb.options.SingleFileMapping && vsmb.options.RestrictFileAccess)
	return !(s.options.RestrictFileAccess && s.options.SingleFileMapping)
}

// GobEncode serializes the Share struct.
func (s *Share) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	errMsgFmt := "failed to encode Share: %s"
	// encode only the fields that can be safely deserialized.
	// Always use vsmbCurrentSerialVersionID as vsmb.serialVersionID might not have
	// been initialized.
	if err := encoder.Encode(CurrentSerialVersionID); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(s.HostPath); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(s.name); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(s.allowedFiles); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(s.guestPath); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(s.options); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	return buf.Bytes(), nil
}

// GobDecode deserializes the Share struct into the struct on which this is called
// (i.e the vsmb pointer).
func (s *Share) GobDecode(data []byte) error {
	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)
	errMsgFmt := "failed to decode Share: %s"
	// fields should be decoded in the same order in which they were encoded.
	// And verify the serialVersionID first
	if err := decoder.Decode(&s.serialVersionID); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if s.serialVersionID != CurrentSerialVersionID {
		return fmt.Errorf(
			"serialized version of Share %d doesn't match with the current version %d",
			s.serialVersionID,
			CurrentSerialVersionID)
	}
	if err := decoder.Decode(&s.HostPath); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if err := decoder.Decode(&s.name); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if err := decoder.Decode(&s.allowedFiles); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if err := decoder.Decode(&s.guestPath); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if err := decoder.Decode(&s.options); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	return nil
}

// Clone creates a clone of the Share `vsmb` and adds that clone to the uvm `vm`.  To
// clone a vSMB share we just need to add it into the config doc of that VM and increase the
// vSMB counter.
func (s *Share) Clone(_ context.Context, uvm vm.UVM, cd *vm.CloneData) error {
	// prevent any updates to the original vsmb
	s.m.mu.RLock()
	defer s.m.mu.RUnlock()

	// lock the clone uVM's vSMB controller for writing
	// if `vsmb.m.host == vm`, bad things will happen
	vm.vsmb.mu.Lock()
	defer vm.vsmb.mu.Unlock()

	cd.Doc.VirtualMachine.Devices.VirtualSmb.Shares = append(cd.Doc.VirtualMachine.Devices.VirtualSmb.Shares, hcsschema.VirtualSmbShare{
		Name:         s.name,
		Path:         s.HostPath,
		Options:      &s.options,
		AllowedFiles: s.allowedFiles,
	})
	vm.vsmb.counter++

	clonedVSMB := &Share{
		m:               vm.vsmb,
		HostPath:        s.HostPath,
		refCount:        1,
		name:            s.name,
		options:         s.options,
		allowedFiles:    s.allowedFiles,
		guestPath:       s.guestPath,
		serialVersionID: CurrentSerialVersionID,
	}
	shareKey := s.shareKey()
	m := vm.vsmb.getShareMap(s.isDirShare())
	m[shareKey] = clonedVSMB
	return nil
}

func (*Share) GetSerialVersionID() uint32 {
	return CurrentSerialVersionID
}

func (s *Share) shareKey() string {
	return getShareKey(s.HostPath, s.options.ReadOnly)
}

// getShareKey returns a string key which encapsulates the information that is used to
// look up an existing VSMB share. If a share is being added, but there is an existing
// share with the same key, the existing share will be used instead (and its ref count
// incremented).
func getShareKey(hostPath string, readOnly bool) string {
	return fmt.Sprintf("%v-%v", hostPath, readOnly)
}
