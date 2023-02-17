//go:build windows

package vsmb

import (
	"context"
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
	"github.com/Microsoft/hcsshim/internal/winapi"
	"github.com/Microsoft/hcsshim/osversion"
)

const SharePrefix = `\\?\VMSMB\VSMB-{dcc079ae-60ba-4d07-847c-3493609c0870}\`

// vsmbMapping map the host directory into the uVM to the [Share] itself.
// A convenience alias.
type vsmbMapping = map[string]*Share

// Manager tracks and manages virtual SMB (vSMB) shares.
type Manager struct {
	// Indicates if VSMB devices should be added with the `NoDirectMap` option.
	//
	// Readonly for manager's lifespan
	noDirectMap bool

	// noWritableFileShares disables mounting any writable plan9 shares.
	// This prevents the guest from modifying files and directories shared into it.
	//
	// Readonly for manager's lifespan
	noWritableFileShares bool

	// mu locks the following fields
	mu sync.RWMutex

	// parent uVM; used to send modify requests over GCS
	host resource.Host

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

var _ resource.CloneManager[*Share] = &Manager{}

func NewManager(host resource.Host, noDirectMap, noWritableFileShares bool) *Manager {
	return &Manager{
		host:                 host,
		dirShares:            make(vsmbMapping),
		fileShares:           make(vsmbMapping),
		noDirectMap:          noDirectMap,
		noWritableFileShares: noWritableFileShares,
	}
}

func (m *Manager) Host() (resource.Host, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if err := m.validate(); err != nil {
		return nil, err
	}

	return m.host, nil
}

// Add adds a virtual SMB share to a running Windows resource host.
// Each vSMB share is ref-counted and only added if it isn't already.
// This is used for read-only layers, mapped directories to a container, and for mapped pipes.
func (m *Manager) Add(ctx context.Context, hostPath string, options *hcsschema.VirtualSmbShareOptions) (*Share, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.entry(ctx).WithField(logfields.Path, hostPath)
	entry.Trace("Adding vSMB share")

	if !options.ReadOnly && m.noWritableFileShares {
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
			entry.Info("Disabling DirectMap for vSMB share")
			options.NoDirectmap = true
		}
	}

	if err := m.validate(); err != nil {
		return nil, err
	}
	sm := m.getShareMap(!isDir)

	requestType := guestrequest.RequestTypeUpdate
	shareKey := shareKey(hostPath, options.ReadOnly)
	share, err := m.get(shareKey, isDir)
	if errors.Is(err, resource.ErrNotAttached) {
		requestType = guestrequest.RequestTypeAdd
		m.counter++
		shareName := "s" + strconv.FormatUint(m.counter, 16)

		share = &Share{
			m:               m,
			name:            shareName,
			guestPath:       SharePrefix + shareName,
			hostPath:        hostPath,
			serialVersionID: currentSerialVersionID,
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
		if err := m.host.Modify(ctx, modification); err != nil {
			return nil, err
		}
	}

	share.allowedFiles = newAllowedFiles
	share.refCount++
	share.options = *options
	sm[shareKey] = share
	return share, nil
}

// Close releases any vSMB shares still remaining and clears the manager's internal state.
func (m *Manager) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.entry(ctx)
	entry.Trace("Closing vSMB manager")

	if err := m.validate(); err != nil {
		return fmt.Errorf("vSMB manager was already closed: %w", hcs.ErrAlreadyClosed)
	}

	// todo: multierror to aggregate and return removeShare() errors
	for _, share := range m.list() {
		if err := m.remove(ctx, share); err != nil {
			entry.WithFields(logrus.Fields{
				logrus.ErrorKey: err,
				logfields.Path:  share.hostPath,
			}).Warning("could not remove vSMB share")
		}
	}
	m.dirShares = nil
	m.fileShares = nil
	m.host = nil
	return nil
}

// FindAndRemove finds then removes a virtual SMB share, preventing intermediate operations until
// it is complete.
// Each VSMB share is ref-counted and only actually removed when the ref-count drops to zero.
func (m *Manager) FindAndRemove(ctx context.Context, hostPath string, readOnly bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.validate(); err != nil {
		return err
	}

	r, err := m.find(hostPath, readOnly)
	if err != nil {
		return err
	}
	return m.remove(ctx, r)
}

// Remove removes a virtual SMB share.
// Each VSMB share is ref-counted and only actually removed when the ref-count drops to zero.
func (m *Manager) Remove(ctx context.Context, share *Share) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.validate(); err != nil {
		return err
	}

	return m.remove(ctx, share)
}

// must hold m.mu before calling.
func (m *Manager) remove(ctx context.Context, r *Share) error {
	entry := m.entry(ctx).WithFields(logrus.Fields{
		logfields.Path: r.hostPath,
		"name":         r.name,
		"refCount":     r.refCount,
	})
	entry.Trace("Removing vSMB share")

	r.refCount--
	if r.refCount > 0 {
		entry.Debug("vSMB share still in use")
		return nil
	}

	modification := &hcsschema.ModifySettingRequest{
		RequestType:  guestrequest.RequestTypeRemove,
		Settings:     hcsschema.VirtualSmbShare{Name: r.name},
		ResourcePath: resourcepaths.VSMBShareResourcePath,
	}
	if err := m.host.Modify(ctx, modification); err != nil {
		return fmt.Errorf("vSMB remove request %+v for share %q in %q failed: %w",
			modification, r.hostPath, m.host.ID(), err)
	}

	sm := m.getShareMap(r.isDirShare())
	key := r.shareKey()
	delete(sm, key)

	return r.close(ctx)
}

// List returns all the vSMB shares the manager currently holds.
func (m *Manager) List(ctx context.Context) ([]*Share, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if err := m.validate(); err != nil {
		return nil, err
	}

	m.entry(ctx).Trace("Listing vSMB shares")

	return m.list(), nil
}

// must hold m.mu before calling.
func (m *Manager) list() []*Share {
	rs := make([]*Share, 0, len(m.dirShares)+len(m.fileShares))
	for _, r := range m.dirShares {
		rs = append(rs, r)
	}
	for _, r := range m.fileShares {
		rs = append(rs, r)
	}
	return rs
}

// GetUVMPath returns the guest path of a vSMB share.
func (m *Manager) GetUVMPath(ctx context.Context, hostPath string, readOnly bool) (string, error) {
	share, err := m.Find(ctx, hostPath, readOnly)
	if err != nil {
		return "", err
	}
	_, f := filepath.Split(hostPath)
	return filepath.Join(share.guestPath, f), nil
}

// Find looks for the vSMB share for the file or directory hostPath.
//
// If not found, it returns `ErrNotAttached`.
func (m *Manager) Find(ctx context.Context, hostPath string, readOnly bool) (*Share, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if err := m.validate(); err != nil {
		return nil, err
	}

	return m.find(hostPath, readOnly)
}

func (m *Manager) find(p string, readOnly bool) (*Share, error) {
	if p == "" {
		return nil, fmt.Errorf("empty hostPath")
	}

	p, isDir, err := processHostPath(p)
	if err != nil {
		return nil, err
	}
	k := shareKey(p, readOnly)
	return m.get(k, isDir)
}

func (m *Manager) get(k string, isDir bool) (*Share, error) {
	sm := m.getShareMap(isDir)
	if share, ok := sm[k]; ok {
		return share, nil
	}
	return nil, resource.ErrNotAttached
}

// getShareMap returns either the [vsmbMapping] of file or directory shares, depending
// on fileShare.
//
// must hold m.mu before calling.
func (m *Manager) getShareMap(isDir bool) vsmbMapping {
	if isDir {
		return m.dirShares
	}
	return m.fileShares
}

// Clone creates a clone of the Share `vsmb` and adds that clone to the uvm `vm`.  To
// clone a vSMB share we just need to add it into the config doc of that VM and increase the
// vSMB counter.
func (m *Manager) Clone(_ context.Context, share *Share, cd *resource.CloneData) error {
	// lock the clone uVM's vSMB controller for writing
	// if `vsmb.m.host == vm`, bad things will happen
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.validate(); err != nil {
		return err
	}

	cd.Doc.VirtualMachine.Devices.VirtualSmb.Shares = append(cd.Doc.VirtualMachine.Devices.VirtualSmb.Shares, hcsschema.VirtualSmbShare{
		Name:         share.name,
		Path:         share.hostPath,
		Options:      &share.options,
		AllowedFiles: share.allowedFiles,
	})
	m.counter++

	clonedVSMB := &Share{
		m:               m,
		hostPath:        share.hostPath,
		refCount:        1,
		name:            share.name,
		options:         share.options,
		allowedFiles:    share.allowedFiles,
		guestPath:       share.guestPath,
		serialVersionID: currentSerialVersionID,
	}
	shareKey := share.shareKey()
	sm := m.getShareMap(share.isDirShare())
	sm[shareKey] = clonedVSMB
	return nil
}

// DefaultOptions returns the default VSMB options. If readOnly is specified,
// returns the default VSMB options for a readonly share.
func (m *Manager) DefaultOptions(readOnly bool) *hcsschema.VirtualSmbShareOptions {
	opts := &hcsschema.VirtualSmbShareOptions{
		NoDirectmap: m.host.DevicesPhysicallyBacked() || m.noDirectMap,
	}
	if readOnly {
		opts.ShareRead = true
		opts.CacheIo = true
		opts.ReadOnly = true
		opts.PseudoOplocks = true
	}
	return opts
}

func (*Manager) SetSaveableOptions(opts *hcsschema.VirtualSmbShareOptions, readOnly bool) {
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

// must hold m.mu before calling.
func (m *Manager) entry(ctx context.Context) *logrus.Entry {
	e := log.G(ctx)
	if m.invalid() {
		return e
	}
	return e.WithFields(logrus.Fields{
		logfields.UVMID: m.host.ID(),
	})
}

// must hold m.mu before calling.
func (m *Manager) validate() error {
	if m.invalid() {
		return fmt.Errorf("vSMB manager not initialized or already closed: %w", resource.ErrInvalidManagerState)
	}
	return nil
}

// must hold m.mu before calling.
func (m *Manager) invalid() bool { return m == nil || m.host == nil }

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
	// todo: should m.allowedFiles be a set[string] (map[string]struct{})? or sorted?
	for _, ss := range a {
		if s == ss {
			return a
		}
	}
	return append(a, s)
}
