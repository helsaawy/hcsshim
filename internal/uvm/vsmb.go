//go:build windows

package uvm

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
	"github.com/Microsoft/hcsshim/internal/winapi"
	"github.com/Microsoft/hcsshim/osversion"
)

const (
	vsmbSharePrefix                   = `\\?\VMSMB\VSMB-{dcc079ae-60ba-4d07-847c-3493609c0870}\`
	vsmbCurrentSerialVersionID uint32 = 1
)

// DefaultVSMBOptions returns the default VSMB options. If readOnly is specified,
// returns the default VSMB options for a readonly share.
func (uvm *UtilityVM) DefaultVSMBOptions(readOnly bool) *hcsschema.VirtualSmbShareOptions {
	opts := &hcsschema.VirtualSmbShareOptions{
		NoDirectmap: uvm.DevicesPhysicallyBacked() || uvm.vsmb.NoDirectMap(),
	}
	if readOnly {
		opts.ShareRead = true
		opts.CacheIo = true
		opts.ReadOnly = true
		opts.PseudoOplocks = true
	}
	return opts
}

func (*UtilityVM) SetSaveableVSMBOptions(opts *hcsschema.VirtualSmbShareOptions, readOnly bool) {
	if readOnly {
		opts.ShareRead = true
		opts.CacheIo = true
		opts.ReadOnly = true
		opts.PseudoOplocks = true
		opts.NoOplocks = false
	} else {
		// Using NoOpLocks can cause intermittent Access denied failures due to
		// a VSMB bug that was fixed but not backported to RS5/19H1.
		opts.ShareRead = false
		opts.CacheIo = false
		opts.ReadOnly = false
		opts.PseudoOplocks = false
		opts.NoOplocks = true
	}
	opts.NoLocks = true
	opts.PseudoDirnotify = true
	opts.NoDirectmap = true
}

// AddVSMB adds a VSMB share to a Windows utility VM. Each VSMB share is ref-counted and
// only added if it isn't already. This is used for read-only layers, mapped directories
// to a container, and for mapped pipes.
func (uvm *UtilityVM) AddVSMB(ctx context.Context, hostPath string, options *hcsschema.VirtualSmbShareOptions) (*VSMBShare, error) {
	entry := log.G(ctx).WithField(logfields.Path, hostPath)
	entry.Trace("Adding VSMB mount")
	if uvm.operatingSystem != "windows" {
		return nil, errNotSupported
	}

	if !options.ReadOnly && uvm.NoWritableFileShares() {
		return nil, fmt.Errorf("adding writable shares is denied: %w", hcs.ErrOperationDenied)
	}

	// Temporary support to allow single-file mapping. If `hostPath` is a
	// directory, map it without restriction. However, if it is a file, map the
	// directory containing the file, and use `AllowedFileList` to only allow
	// access to that file. If the directory has been mapped before for
	// single-file use, add the new file to the `AllowedFileList` and issue an
	// Update operation.
	file := hostPath
	hostPath, isDir, err := processHostPath(hostPath)
	if err != nil {
		return nil, err
	}
	if !isDir {
		options.RestrictFileAccess = true
		options.SingleFileMapping = true
	}

	if !options.NoDirectmap {
		if force, err := forceNoDirectMap(hostPath); err != nil {
			return nil, err
		} else if force {
			entry.Info("Disabling DirectMap for vSMB mount")
			options.NoDirectmap = true
		}
	}

	vsmb := uvm.vsmb

	vsmb.mu.Lock()
	defer vsmb.mu.Unlock()
	m := vsmb.getShareMap(!isDir)

	requestType := guestrequest.RequestTypeUpdate
	shareKey := getVSMBShareKey(hostPath, options.ReadOnly)
	share, err := vsmb.getshare(shareKey, isDir)
	if errors.Is(err, ErrNotAttached) {
		requestType = guestrequest.RequestTypeAdd
		vsmb.counter++
		shareName := "s" + strconv.FormatUint(vsmb.counter, 16)

		share = &VSMBShare{
			c:               vsmb,
			name:            shareName,
			guestPath:       vsmbSharePrefix + shareName,
			HostPath:        hostPath,
			serialVersionID: vsmbCurrentSerialVersionID,
		}
		if !isDir {
			// preallocate some room for growth
			share.allowedFiles = make([]string, 0, 3)
		}
	}
	newAllowedFiles := share.allowedFiles
	//todo: prevent duplicate files, and prevent updating if no new files are added
	if !isDir {
		entry.Debugf("adding %s to %s", file, newAllowedFiles)
		newAllowedFiles = appendIfNotExists(newAllowedFiles, file)
	}

	// Update on a VSMB share currently only supports updating the
	// AllowedFileList, and in fact will return an error if RestrictFileAccess
	// isn't set (e.g. if used on an unrestricted share).
	// So we only call Modify if we are either doing an Add, or if RestrictFileAccess is set.
	//
	// todo: should we skip this if the file already in allowedFiles and just increment refCount?
	// also, is there ever a case where the host path is a directory, but options.RestrictFileAccess
	// is true? Ie, can a share be converted from a directory- to a file-share after creation?
	if requestType == guestrequest.RequestTypeAdd || options.RestrictFileAccess {
		entry.WithFields(logrus.Fields{
			"name":         share.name,
			logfields.Path: hostPath,
			"allowedFiles": newAllowedFiles,
			"options":      fmt.Sprintf("%+#v", options),
			"operation":    requestType,
		}).Debug("Modifying vSMB share")
		modification := &hcsschema.ModifySettingRequest{
			RequestType: requestType,
			Settings: hcsschema.VirtualSmbShare{
				Name:         share.name,
				Options:      options,
				Path:         hostPath,
				AllowedFiles: newAllowedFiles,
			},
			ResourcePath: resourcepaths.VSMBShareResourcePath,
		}
		if err := uvm.modify(ctx, modification); err != nil {
			return nil, err
		}
	}

	share.allowedFiles = newAllowedFiles
	share.refCount++
	share.options = *options
	m[shareKey] = share
	return share, nil
}

// RemoveVSMB removes a VSMB share from a utility VM. Each VSMB share is ref-counted
// and only actually removed when the ref-count drops to zero.
func (uvm *UtilityVM) RemoveVSMB(ctx context.Context, hostPath string, readOnly bool) error {
	entry := log.G(ctx).WithFields(logrus.Fields{
		logfields.Path:  hostPath,
		logfields.UVMID: uvm.id,
	})
	entry.Trace("Removing vSMB mount")

	if uvm.operatingSystem != "windows" {
		return errNotSupported
	}

	share, err := uvm.vsmb.FindShare(hostPath, readOnly)
	if err != nil {
		return fmt.Errorf("%s is not present as a VSMB share in %s, cannot remove", hostPath, uvm.id)
	}

	return uvm.vsmb.RemoveVSMB(ctx, share)
}

// GetVSMBUvmPath returns the guest path of a VSMB mount.
func (uvm *UtilityVM) GetVSMBUvmPath(_ context.Context, hostPath string, readOnly bool) (string, error) {
	return uvm.vsmb.GetUVMPath(hostPath, readOnly)
}

// todo: ideally vsmbManager implement internal/vm/VSMBManager, but the interface requires a unique key per share.

// vsmbManager tracks and manages all vSMB shares in a uVM.
type vsmbManager struct {
	// Indicates if VSMB devices should be added with the `NoDirectMap` option.
	// Readonly for controller lifespan
	noDirectMap bool

	// mu locks the following fields
	mu sync.RWMutex

	// parent uVM; used to send modify requests over GCS
	vm *UtilityVM

	// todo: is updating a file-share the correct choice?
	// If C:\host\foo.txt is shared into uVM as C:\guest\spam\foo.txt, HCS will mount
	// C:\host\ to C:\guest\spam\, and only make foo.txt available.
	// Then, if C:\host\bar.txt is shared into uVM as C:\guest\eggs\bar.txt,
	// ref count will increment, allowedFiles will be updated, and now four files are will be made available:
	//  - C:\guest\spam\foo.txt
	//  - C:\guest\spam\bar.txt
	//  - C:\guest\eggs\foo.txt
	//  - C:\guest\eggs\bar.txt
	//
	// This is likely not the intent at all.
	// However, can multiple vSMB shares be mapped to the same directory?
	// Ie, can we map:
	//  - C:\host\foo.txt -> C:\guest\foo.txt
	//  - C:\host\bar.txt -> C:\guest\bar.txt
	// with two separate vSMB shares?

	// vSMB shares that are mapped into a Windows uVM.
	// These are used for read-only layers and mapped directories.
	// We maintain two sets of maps:
	//  - dirShares tracks shares that are unrestricted mappings of directories
	//  - fileShares tracks shares that  are restricted to some subset of files in the directory.
	// This is used as part of a temporary fix to allow WCOW single-file mapping to function,
	// and to prevent issues when mapping a single file into the uVM, and then attempting to
	// map the entire directory elsewhere in the same uVM.
	dirShares, fileShares vsmbMapping
	// counter to generate a unique share name for each VSMB share.
	counter uint64
}

// vsmbMapping map the host directory into the uVM to the [VSMBShare] itself.
// A convenience alias.
type vsmbMapping = map[string]*VSMBShare

func NewController(vm *UtilityVM, noDirectMap bool) *vsmbManager {
	return &vsmbManager{
		vm:          vm,
		dirShares:   make(vsmbMapping),
		fileShares:  make(vsmbMapping),
		noDirectMap: noDirectMap,
	}
}

// Close releases any vSMB shares still remaining and clears the controllers internal state.
func (m *vsmbManager) Close(ctx context.Context) error {
	entry := log.G(ctx)
	entry.Trace("closing vSMB controller")
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.vm == nil {
		return fmt.Errorf("vSMB controller was already closed: %w", hcs.ErrAlreadyClosed)
	}

	// todo: multierror to aggregate and return removevsmb() errors
	for _, share := range m.shares() {
		if err := m.removevsmb(ctx, share); err != nil {
			entry.WithFields(logrus.Fields{
				logrus.ErrorKey: err,
				logfields.Path:  share.HostPath,
			}).Warning("could not close vSMB share")
		}
	}
	m.dirShares = nil
	m.fileShares = nil
	m.vm = nil
	return nil
}

// RemoveVSMB removes a VSMB share from the uVM.
// Each VSMB share is ref-counted and only actually removed when the ref-count drops to zero.
func (m *vsmbManager) RemoveVSMB(ctx context.Context, share *VSMBShare) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.removevsmb(ctx, share)
}

func (m *vsmbManager) removevsmb(ctx context.Context, share *VSMBShare) error {
	ctx, entry := log.S(ctx, logrus.Fields{
		logfields.Path: share.HostPath,
		"refCount":     share.refCount,
	})
	entry.Trace("Removing vSMB mount")

	share.refCount--
	if share.refCount > 0 {
		entry.Debug("vSMB share still in use")
		return nil
	}

	modification := &hcsschema.ModifySettingRequest{
		RequestType:  guestrequest.RequestTypeRemove,
		Settings:     hcsschema.VirtualSmbShare{Name: share.name},
		ResourcePath: resourcepaths.VSMBShareResourcePath,
	}
	if err := m.vm.modify(ctx, modification); err != nil {
		return fmt.Errorf("vSMB remove request %+v for share %q in %q failed: %w",
			modification, share.HostPath, m.vm.id, err)
	}

	sm := m.getShareMap(share.isDirShare())
	key := share.shareKey()
	delete(sm, key)

	return share.close(ctx)
}

// Shares returns all the shares vSMB shares the controller currently holds.
func (m *vsmbManager) nextKey() uint64 {
	m.counter++
	return m.counter
}

// Shares returns all the shares vSMB shares the controller currently holds.
func (m *vsmbManager) Shares() []*VSMBShare {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.shares()
}

func (m *vsmbManager) shares() []*VSMBShare {
	shares := make([]*VSMBShare, 0, len(m.dirShares)+len(m.fileShares))
	for _, s := range m.dirShares {
		shares = append(shares, s)
	}
	for _, s := range m.fileShares {
		shares = append(shares, s)
	}
	return shares
}

// GetUVMPath returns the guest path of a VSMB mount.
func (m *vsmbManager) GetUVMPath(hostPath string, readOnly bool) (string, error) {
	share, err := m.FindShare(hostPath, readOnly)
	if err != nil {
		return "", err
	}
	_, f := filepath.Split(hostPath)
	return filepath.Join(share.guestPath, f), nil
}

// FindShare looks for the vSMB share for the file or directory hostPath.
//
// If not found, it returns `ErrNotAttached`.
func (m *vsmbManager) FindShare(hostPath string, readOnly bool) (*VSMBShare, error) {
	if hostPath == "" {
		return nil, fmt.Errorf("empty hostPath")
	}

	hostPath, isDir, err := processHostPath(hostPath)
	if err != nil {
		return nil, err
	}
	shareKey := getVSMBShareKey(hostPath, readOnly)

	return m.GetShare(shareKey, isDir)
}

// GetShare returns either the file or directory share (depending on fileShare) with key, k.
//
// If not found, it returns `ErrNotAttached`.
func (m *vsmbManager) GetShare(k string, dirShare bool) (*VSMBShare, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getshare(k, dirShare)
}

func (m *vsmbManager) getshare(k string, dirShare bool) (*VSMBShare, error) {
	sm := m.getShareMap(dirShare)
	if share, ok := sm[k]; ok {
		return share, nil
	}
	return nil, ErrNotAttached
}

// getShareMap returns either the [vsmbMapping] of file or directory shares, depending
// on fileShare.
//
// c.mu should be locked.
func (m *vsmbManager) getShareMap(dirShare bool) vsmbMapping {
	if dirShare {
		return m.dirShares
	}
	return m.fileShares
}

func (m *vsmbManager) NoDirectMap() bool { return m.noDirectMap }

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

// Clone creates a clone of the VSMBShare `vsmb` and adds that clone to the uvm `vm`.  To
// clone a vSMB share we just need to add it into the config doc of that VM and increase the
// vSMB counter.
func (vsmb *VSMBShare) Clone(_ context.Context, vm *UtilityVM, cd *cloneData) error {
	// prevent any updates to the original vsmb
	vsmb.c.mu.RLock()
	defer vsmb.c.mu.RUnlock()

	// lock the clone uVM's vSMB controller for writing
	// if `vsmb.c.vm == vm`, bad things will happen
	vm.vsmb.mu.Lock()
	defer vm.vsmb.mu.Unlock()

	cd.doc.VirtualMachine.Devices.VirtualSmb.Shares = append(cd.doc.VirtualMachine.Devices.VirtualSmb.Shares, hcsschema.VirtualSmbShare{
		Name:         vsmb.name,
		Path:         vsmb.HostPath,
		Options:      &vsmb.options,
		AllowedFiles: vsmb.allowedFiles,
	})
	vm.vsmb.counter++

	clonedVSMB := &VSMBShare{
		c:               vm.vsmb,
		HostPath:        vsmb.HostPath,
		refCount:        1,
		name:            vsmb.name,
		options:         vsmb.options,
		allowedFiles:    vsmb.allowedFiles,
		guestPath:       vsmb.guestPath,
		serialVersionID: vsmbCurrentSerialVersionID,
	}
	shareKey := vsmb.shareKey()
	m := vm.vsmb.getShareMap(vsmb.isDirShare())
	m[shareKey] = clonedVSMB
	return nil
}

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

// openHostPath opens the given path and returns the handle. The handle is opened with
// full sharing and no access mask. The directory must already exist. This
// function is intended to return a handle suitable for use with GetFileInformationByHandleEx.
//
// We are not able to use builtin Go functionality for opening a directory path:
//   - os.Open on a directory returns a os.File where Fd() is a search handle from FindFirstFile.
//   - syscall.Open does not provide a way to specify FILE_FLAG_BACKUP_SEMANTICS, which is needed to
//     open a directory.
//
// We could use os.Open if the path is a file, but it's easier to just use the same code for both.
// Therefore, we call windows.CreateFile directly.
func openHostPath(path string) (windows.Handle, error) {
	u16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	h, err := windows.CreateFile(
		u16,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0)
	if err != nil {
		return 0, &os.PathError{
			Op:   "CreateFile",
			Path: path,
			Err:  err,
		}
	}
	return h, nil
}

// In 19H1, a change was made to VSMB to require querying file ID for the files being shared in
// order to support direct map. This change was made to ensure correctness in cases where direct
// map is used with saving/restoring VMs.
//
// However, certain file systems (such as Azure Files SMB shares) don't support the FileIdInfo
// query that is used. Azure Files in particular fails with ERROR_INVALID_PARAMETER. This issue
// affects at least 19H1, 19H2, 20H1, and 20H2.
//
// To work around this, we attempt to query for FileIdInfo ourselves if on an affected build. If
// the query fails, we override the specified options to force no direct map to be used.
func forceNoDirectMap(path string) (bool, error) {
	if ver := osversion.Build(); ver < osversion.V19H1 || ver > osversion.V20H2 {
		return false, nil
	}
	h, err := openHostPath(path)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = windows.CloseHandle(h)
	}()
	var info winapi.FILE_ID_INFO
	// We check for any error, rather than just ERROR_INVALID_PARAMETER. It seems better to also
	// fall back if e.g. some other backing filesystem is used which returns a different error.
	if err := windows.GetFileInformationByHandleEx(
		h,
		winapi.FileIdInfo,
		(*byte)(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		//nolint:nilerr // error is expected and can safely be ignored
		return true, nil
	}
	return false, nil
}

// processHostPath cleans the path p, and if p is not a directory, returns the file's
// parent directory as well as true.
// For directories, it returns the cleaned path and false.
func processHostPath(p string) (string, bool, error) {
	isDir, err := isPathDir(p)
	if err != nil {
		return "", false, err
	}
	if !isDir {
		p = filepath.Dir(p)
	}
	return filepath.Clean(p), isDir, nil
}

// an arbitrary file, f, can have both os.Stat(f).Mode().IsDir & os.Stat(f).Mode().IsRegular()
// be false.
// So, here we say something is a file (share) if it is not a directory.

func isPathDir(p string) (bool, error) {
	st, err := os.Stat(p)
	if err != nil {
		return false, err
	}
	return st.IsDir(), nil
}

// appendIfNotExists appends s to a iff a does not contain a string equal to s.
func appendIfNotExists(a []string, s string) []string {
	// todo: should vsmb.allowedFiles be a set[string] (map[string]struct{})? or sorted?
	for _, ss := range a {
		if s == ss {
			return a
		}
	}
	return append(a, s)
}
