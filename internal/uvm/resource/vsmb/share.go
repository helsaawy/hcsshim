//go:build windows

package vsmb

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"unsafe"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"

	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/hcs/resourcepaths"
	hcsschema "github.com/Microsoft/hcsshim/internal/hcs/schema2"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/Microsoft/hcsshim/internal/protocol/guestrequest"
	"github.com/Microsoft/hcsshim/internal/uvm/resource"
	"github.com/Microsoft/hcsshim/internal/vm"
	"github.com/Microsoft/hcsshim/internal/winapi"
	"github.com/Microsoft/hcsshim/osversion"
)

// VSMBShare contains the host path for a vSMB mount.
//
// Do not modify unless holding parent controller lock, c.mu.
type VSMBShare struct {
	// controller the resource belongs to
	c *vsmbManager

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

var _ Cloneable = &VSMBShare{}

// Release frees the resources of the corresponding vSMB mount.
func (vsmb *VSMBShare) Release(ctx context.Context) error {
	if err := vsmb.c.RemoveVSMB(ctx, vsmb); err != nil {
		return fmt.Errorf("failed to release vSMB share: %w", err)
	}
	return nil
}

// close closes a vSMB share. It is unsynchronized and should only be called when already
// the the controller lock
func (vsmb *VSMBShare) close(_ context.Context) error {
	if vsmb.c == nil {
		return fmt.Errorf("vSMB share already closed: %w", hcs.ErrAlreadyClosed)
	}
	vsmb.HostPath = ""
	vsmb.allowedFiles = nil
	vsmb.options = hcsschema.VirtualSmbShareOptions{}
	vsmb.c = nil
	return nil
}

func (vsmb *VSMBShare) isDirShare() bool {
	// allowedFiles != nil iff (vsmb.options.SingleFileMapping && vsmb.options.RestrictFileAccess)
	return !(vsmb.options.RestrictFileAccess && vsmb.options.SingleFileMapping)
}

// GobEncode serializes the VSMBShare struct.
func (vsmb *VSMBShare) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	errMsgFmt := "failed to encode VSMBShare: %s"
	// encode only the fields that can be safely deserialized.
	// Always use vsmbCurrentSerialVersionID as vsmb.serialVersionID might not have
	// been initialized.
	if err := encoder.Encode(vsmbCurrentSerialVersionID); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(vsmb.HostPath); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(vsmb.name); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(vsmb.allowedFiles); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(vsmb.guestPath); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	if err := encoder.Encode(vsmb.options); err != nil {
		return nil, fmt.Errorf(errMsgFmt, err)
	}
	return buf.Bytes(), nil
}

// GobDecode deserializes the VSMBShare struct into the struct on which this is called
// (i.e the vsmb pointer).
func (vsmb *VSMBShare) GobDecode(data []byte) error {
	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)
	errMsgFmt := "failed to decode VSMBShare: %s"
	// fields should be decoded in the same order in which they were encoded.
	// And verify the serialVersionID first
	if err := decoder.Decode(&vsmb.serialVersionID); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if vsmb.serialVersionID != vsmbCurrentSerialVersionID {
		return fmt.Errorf(
			"serialized version of VSMBShare %d doesn't match with the current version %d",
			vsmb.serialVersionID,
			vsmbCurrentSerialVersionID)
	}
	if err := decoder.Decode(&vsmb.HostPath); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if err := decoder.Decode(&vsmb.name); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if err := decoder.Decode(&vsmb.allowedFiles); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if err := decoder.Decode(&vsmb.guestPath); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	if err := decoder.Decode(&vsmb.options); err != nil {
		return fmt.Errorf(errMsgFmt, err)
	}
	return nil
}

// // Clone creates a clone of the VSMBShare `vsmb` and adds that clone to the uvm `vm`.  To
// // clone a vSMB share we just need to add it into the config doc of that VM and increase the
// // vSMB counter.
// func (vsmb *VSMBShare) Clone(_ context.Context, vm *UtilityVM, cd *cloneData) error {
// 	// prevent any updates to the original vsmb
// 	vsmb.c.mu.RLock()
// 	defer vsmb.c.mu.RUnlock()

// 	// lock the clone uVM's vSMB controller for writing
// 	// if `vsmb.c.vm == vm`, bad things will happen
// 	vm.vsmb.mu.Lock()
// 	defer vm.vsmb.mu.Unlock()

// 	cd.doc.VirtualMachine.Devices.VirtualSmb.Shares = append(cd.doc.VirtualMachine.Devices.VirtualSmb.Shares, hcsschema.VirtualSmbShare{
// 		Name:         vsmb.name,
// 		Path:         vsmb.HostPath,
// 		Options:      &vsmb.options,
// 		AllowedFiles: vsmb.allowedFiles,
// 	})
// 	vm.vsmb.counter++

// 	clonedVSMB := &VSMBShare{
// 		c:               vm.vsmb,
// 		HostPath:        vsmb.HostPath,
// 		refCount:        1,
// 		name:            vsmb.name,
// 		options:         vsmb.options,
// 		allowedFiles:    vsmb.allowedFiles,
// 		guestPath:       vsmb.guestPath,
// 		serialVersionID: vsmbCurrentSerialVersionID,
// 	}
// 	shareKey := vsmb.shareKey()
// 	m := vm.vsmb.getShareMap(vsmb.isDirShare())
// 	m[shareKey] = clonedVSMB
// 	return nil
// }

func (*VSMBShare) GetSerialVersionID() uint32 {
	return vsmbCurrentSerialVersionID
}

func (vsmb *VSMBShare) shareKey() string {
	return getVSMBShareKey(vsmb.HostPath, vsmb.options.ReadOnly)
}

// getVSMBShareKey returns a string key which encapsulates the information that is used to
// look up an existing VSMB share. If a share is being added, but there is an existing
// share with the same key, the existing share will be used instead (and its ref count
// incremented).
func getVSMBShareKey(hostPath string, readOnly bool) string {
	return fmt.Sprintf("%v-%v", hostPath, readOnly)
}
