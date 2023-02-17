//go:build windows

package scsi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/Microsoft/hcsshim/internal/copyfile"
	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/hcs/resourcepaths"
	hcsschema "github.com/Microsoft/hcsshim/internal/hcs/schema2"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/Microsoft/hcsshim/internal/protocol/guestrequest"
	"github.com/Microsoft/hcsshim/internal/protocol/guestresource"
	"github.com/Microsoft/hcsshim/internal/security"
	"github.com/Microsoft/hcsshim/internal/uvm/resource"
	"github.com/Microsoft/hcsshim/internal/wclayer"
)

var (
	ErrNoSCSIControllers        = fmt.Errorf("no SCSI controllers configured for this utility VM: %w", resource.ErrInvalidManagerState)
	ErrTooManyAttachments       = fmt.Errorf("too many SCSI attachments: %w", resource.ErrNoAvailableLocation)
	ErrSCSILayerWCOWUnsupported = fmt.Errorf("SCSI attached layers are not supported for WCOW: %w", resource.ErrNotSupported)
)

// Manager tracks and manages Small Computer System Interface (SCSI) drive mounts.
type Manager struct {
	// mu locks the following fields
	mu sync.RWMutex

	// parent uVM; used to send modify requests over GCS
	host resource.Host

	// Hyper-V supports 4 controllers, 64 slots per controller. Limited to 1 controller for now though.
	locations       [4][64]*Mount
	controllerCount uint32 // Number of SCSI controllers in the utility VM
}

var _ resource.CloneManager[*Mount] = &Manager{}

func NewManager(host resource.Host, controllerCount uint32) *Manager {
	return &Manager{
		host:            host,
		controllerCount: controllerCount,
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

func (m *Manager) ControllerCount() uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.invalid() {
		return 0
	}

	return m.controllerCount
}

// Add is the implementation behind the external functions AddSCSI,
// AddSCSIPhysicalDisk, AddSCSIExtensibleVirtualDisk.
//
// We are in control of everything ourselves. Hence we have ref-counting and
// so-on tracking what SCSI locations are available or used.
//
// Returns result from calling modify with the given scsi mount
func (m *Manager) Add(ctx context.Context, req *AddRequest) (_ *Mount, err error) {
	// We must hold the lock throughout the lookup (m.find) until
	// after the possible allocation (allocateSlot) has been completed to ensure
	// there isn't a race condition for it being attached by another thread between
	// these two operations.
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.entry(ctx).WithFields(logrus.Fields{
		"HostPath":       req.HostPath,
		"UVMPath":        req.UVMPath,
		"AttachmentType": string(req.AttachmentType),
		"Encrypted":      req.Encrypted,
		"ReadOnly":       req.ReadOnly,
	})
	entry.Trace("Adding SCSI mount")

	if err := m.validate(); err != nil {
		return nil, err
	}

	sm, existed, err := m.allocateMount(
		ctx,
		req.ReadOnly,
		req.Encrypted,
		req.HostPath,
		req.UVMPath,
		req.AttachmentType,
		req.EVDType,
		req.VMAccess,
	)
	if err != nil {
		return nil, err
	}

	if existed {
		// another mount request might be in progress, wait for it to finish and if that operation
		// fails return that error.
		<-sm.waitCh
		if sm.waitErr != nil {
			return nil, sm.waitErr
		}
		return sm, nil
	}

	// This is the first goroutine to add this disk, close the waitCh after we are done.
	defer func() {
		if err != nil {
			m.deallocateSlot(ctx, sm)
		}

		// error must be set _before_ the channel is closed.
		sm.waitErr = err
		close(sm.waitCh)
	}()

	SCSIModification := &hcsschema.ModifySettingRequest{
		RequestType: guestrequest.RequestTypeAdd,
		Settings: hcsschema.Attachment{
			Path:                      sm.hostPath,
			Type_:                     string(req.AttachmentType),
			ReadOnly:                  req.ReadOnly,
			ExtensibleVirtualDiskType: req.EVDType,
		},
		ResourcePath: fmt.Sprintf(resourcepaths.SCSIResourceFormat, guestrequest.ScsiControllerGuids[sm.controller], sm.lun),
	}

	if sm.guestPath != "" {
		guestReq := guestrequest.ModificationRequest{
			ResourceType: guestresource.ResourceTypeMappedVirtualDisk,
			RequestType:  guestrequest.RequestTypeAdd,
		}

		if m.host.OS() == "windows" {
			guestReq.Settings = guestresource.WCOWMappedVirtualDisk{
				ContainerPath: sm.guestPath,
				Lun:           sm.lun,
			}
		} else {
			var verity *guestresource.DeviceVerityInfo
			if v, iErr := resource.ReadVeritySuperBlock(ctx, sm.hostPath); iErr != nil {
				log.G(ctx).WithError(iErr).WithField("hostPath", sm.hostPath).Debug("unable to read dm-verity information from VHD")
			} else {
				if v != nil {
					log.G(ctx).WithFields(logrus.Fields{
						"hostPath":   sm.hostPath,
						"rootDigest": v.RootDigest,
					}).Debug("adding SCSI with dm-verity")
				}
				verity = v
			}

			guestReq.Settings = guestresource.LCOWMappedVirtualDisk{
				MountPath:  sm.guestPath,
				Lun:        uint8(sm.lun),
				Controller: uint8(sm.controller),
				ReadOnly:   req.ReadOnly,
				Encrypted:  req.Encrypted,
				Options:    req.GuestOptions,
				VerityInfo: verity,
			}
		}
		SCSIModification.GuestRequest = guestReq
	}

	if err := m.host.Modify(ctx, SCSIModification); err != nil {
		return nil, fmt.Errorf("failed to modify UVM with new SCSI mount: %s", err)
	}
	return sm, nil
}

// allocateMount grants vm access to hostpath and increments the ref count of an existing scsi
// device or allocates a new one if not already present.
// Returns the resulting *Mount, a bool indicating if the scsi device was already present,
// and error if any.
//
// must hold m.mu before calling.
func (m *Manager) allocateMount(
	ctx context.Context,
	readOnly bool,
	encrypted bool,
	hostPath string,
	uvmPath string,
	attachmentType AttachmentType,
	evdType string,
	vmAccess VMAccessType,
) (*Mount, bool, error) {
	if attachmentType != ExtensibleVirtualDiskAttachment {
		if uvmPath == "" {
			return nil, false, errors.New("uvmPath can not be empty for extensible virtual disk")
		}

		// Ensure the utility VM has access
		err := grantAccess(ctx, m.host.ID(), hostPath, vmAccess)
		if err != nil {
			return nil, false, errors.Wrapf(err, "failed to grant VM access for SCSI mount")
		}
	}

	if sm, err := m.find(ctx, hostPath); err == nil {
		sm.refCount++
		return sm, true, nil
	}

	controller, lun, err := m.allocateSlot(ctx)
	if err != nil {
		return nil, false, err
	}

	r := m.NewMount(
		hostPath,
		uvmPath,
		attachmentType,
		evdType,
		1,
		controller,
		int32(lun),
		readOnly,
		encrypted,
	)
	r.entry(ctx).Debug("allocated SCSI mount")

	m.locations[controller][lun] = r
	return r, false, nil
}

// allocateSlot finds the next available slot on the
// SCSI controllers associated with a utility VM to use.
//
// must hold m.mu before calling.
func (m *Manager) allocateSlot(ctx context.Context) (int, int, error) {
	for controller := 0; controller < int(m.controllerCount); controller++ {
		for lun, sm := range m.locations[controller] {
			// If sm is nil, we have found an open slot so we allocate a new Mount
			if sm == nil {
				return controller, lun, nil
			}
		}
	}
	return -1, -1, ErrTooManyAttachments
}

// SetRootMount explicitly sets the (0,0) (Controller,LUN) location to mount.
// A [resource.ErrInvalidManagerState] error is returned if a mount at that location already exists.
func (m *Manager) SetRootMount(mount *Mount) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.validate(); err != nil {
		return err
	}

	if m.locations[0][0] != nil {
		return fmt.Errorf("a mount already exists at controller 0, LUN 0: %w", resource.ErrInvalidManagerState)
	}

	m.locations[0][0] = mount
	return nil
}

// FindAndRemove finds then removes a SCSI disk, preventing intermediate operations until
// it is complete.
func (m *Manager) FindAndRemove(ctx context.Context, hostPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.validate(); err != nil {
		return err
	}

	r, err := m.find(ctx, hostPath)
	if err != nil {
		return err
	}
	return m.remove(ctx, r)
}

// RemoveSCSI removes a SCSI disk from a utility VM.
func (m *Manager) Remove(ctx context.Context, mount *Mount) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.validate(); err != nil {
		return err
	}

	if m.controllerCount == 0 {
		return ErrNoSCSIControllers
	}

	return m.remove(ctx, mount)
}

// must hold m.mu before calling.
func (m *Manager) remove(ctx context.Context, r *Mount) error {
	entry := m.entry(ctx).WithFields(logrus.Fields{
		logfields.Path: r.hostPath,
		"refCount":     r.refCount,
	})
	entry.Trace("Removing SCSI mount")

	if m != r.m {
		// this should definitely not happen
		return fmt.Errorf("SCSI mount belongs to different manager: %w", resource.ErrInvalidResourceState)
	}

	hostPath := r.hostPath
	r.refCount--
	if r.refCount > 0 {
		return nil
	}

	scsiModification := &hcsschema.ModifySettingRequest{
		RequestType:  guestrequest.RequestTypeRemove,
		ResourcePath: fmt.Sprintf(resourcepaths.SCSIResourceFormat, guestrequest.ScsiControllerGuids[r.controller], r.lun),
	}

	var verity *guestresource.DeviceVerityInfo
	if v, iErr := resource.ReadVeritySuperBlock(ctx, hostPath); iErr != nil {
		entry.WithError(iErr).Debug("unable to read dm-verity information from VHD")
	} else {
		if v != nil {
			entry.WithField("rootDigest", v.RootDigest).Debug("removing SCSI with dm-verity")
		}
		verity = v
	}

	// Include the GuestRequest so that the GCS ejects the disk cleanly if the
	// disk was attached/mounted
	//
	// Note: We always send a guest eject even if there is no UVM path in lcow
	// so that we synchronize the guest state. This seems to always avoid SCSI
	// related errors if this index quickly reused by another container.
	if m.host.OS() == "windows" && r.guestPath != "" {
		scsiModification.GuestRequest = guestrequest.ModificationRequest{
			ResourceType: guestresource.ResourceTypeMappedVirtualDisk,
			RequestType:  guestrequest.RequestTypeRemove,
			Settings: guestresource.WCOWMappedVirtualDisk{
				ContainerPath: r.guestPath,
				Lun:           r.lun,
			},
		}
	} else {
		scsiModification.GuestRequest = guestrequest.ModificationRequest{
			ResourceType: guestresource.ResourceTypeMappedVirtualDisk,
			RequestType:  guestrequest.RequestTypeRemove,
			Settings: guestresource.LCOWMappedVirtualDisk{
				MountPath:  r.guestPath, // May be blank in attach-only
				Lun:        uint8(r.lun),
				Controller: uint8(r.controller),
				VerityInfo: verity,
			},
		}
	}

	if err := m.host.Modify(ctx, scsiModification); err != nil {
		return fmt.Errorf("failed to remove SCSI disk %s from container %s: %s", hostPath, m.host.ID(), err)
	}
	m.deallocateSlot(ctx, r)
	return r.close(ctx)
}

// deallocateSlot removes deletes the manager's reference to the Mount at its current (Controller, Lun) location.
//
// must hold m.mu before calling.
func (m *Manager) deallocateSlot(ctx context.Context, r *Mount) {
	if r != nil {
		r.entry(ctx).Debug("removed SCSI location")
		m.locations[r.controller][r.lun] = nil
	}
}

// Close releases any SCSI shares still remaining and clears the manager's internal state.
func (m *Manager) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.entry(ctx)
	entry.Trace("Closing SCSI manager")

	if err := m.validate(); err != nil {
		return fmt.Errorf("SCSI manager was already closed: %w", hcs.ErrAlreadyClosed)
	}

	// todo: multierror to aggregate and return removeShare() errors
	for _, r := range m.list() {
		if err := m.remove(ctx, r); err != nil {
			entry.WithFields(logrus.Fields{
				logrus.ErrorKey: err,
				logfields.Path:  r.hostPath,
			}).Warning("could not remove SCSI share")
		}
	}
	m.controllerCount = 0
	m.host = nil
	return nil
}

// List returns all the SCSI mounts the manager currently holds.
func (m *Manager) List(ctx context.Context) ([]*Mount, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if err := m.validate(); err != nil {
		return nil, err
	}

	m.entry(ctx).Trace("Listing SCSI mounts")
	return m.list(), nil
}

// must hold m.mu before calling.
func (m *Manager) list() []*Mount {
	rs := make([]*Mount, 0, m.controllerCount)
	for _, aa := range m.locations {
		for _, r := range aa {
			if r != nil {
				rs = append(rs, r)
			}
		}
	}
	return rs
}

// GetUVMPath returns the guest mounted path of a SCSI drive.
//
// If `hostPath` is not mounted returns `ErrNotAttached`.
func (m *Manager) GetUVMPath(ctx context.Context, hostPath string) (string, error) {
	r, err := m.Find(ctx, hostPath)
	if err != nil {
		return "", err
	}
	return r.guestPath, err
}

// Find returns the SCSI drive mounted at hostPath.
//
// If `hostPath` is not mounted returns `ErrNotAttached`.
func (m *Manager) Find(ctx context.Context, hostPath string) (*Mount, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if err := m.validate(); err != nil {
		return nil, err
	}

	return m.find(ctx, hostPath)
}

// must hold m.mu before calling.
func (m *Manager) find(ctx context.Context, hostPath string) (*Mount, error) {
	for _, luns := range m.locations {
		for _, r := range luns {
			if r != nil && r.hostPath == hostPath {
				r.entry(ctx).Debug("found SCSI location")
				return r, nil
			}
		}
	}
	return nil, resource.ErrNotAttached
}

// Clone function creates a clone of the Mount `sm` and adds the cloned Mount to
// the uvm `vm`. If `sm` is read only then it is simply added to the `vm`. But if it is a
// writable mount(e.g a scratch layer) then a copy of it is made and that copy is added
// to the `vm`.
func (m *Manager) Clone(ctx context.Context, mount *Mount, cd *resource.CloneData) error {
	var (
		err        error
		dir        string
		dstVhdPath = mount.hostPath
		conStr     = guestrequest.ScsiControllerGuids[mount.controller]
		lunStr     = fmt.Sprintf("%d", mount.lun)
	)

	m.mu.Lock()
	defer m.mu.Unlock()

	if !mount.readOnly {
		// This is a writable SCSI mount. It must be either the
		// 1. scratch VHD of the UVM or
		// 2. scratch VHD of the container.
		// A user provided writable SCSI mount is not allowed on the template UVM
		// or container and so this SCSI mount has to be the scratch VHD of the
		// UVM or container.  The container inside this UVM will automatically be
		// cloned here when we are cloning the uvm itself. We will receive a
		// request for creation of this container later and that request will
		// specify the storage path for this container.  However, that storage
		// location is not available now so we just use the storage path of the
		// uvm instead.
		// TODO(ambarve): Find a better way for handling this. Problem with this
		// approach is that the scratch VHD of the container will not be
		// automatically cleaned after container exits. It will stay there as long
		// as the UVM keeps running.

		// For the scratch VHD of the VM (always attached at Controller:0, LUN:0)
		// clone it in the scratch folder
		dir = cd.ScratchFolder
		if mount.controller != 0 || mount.lun != 0 {
			dir, err = os.MkdirTemp(cd.ScratchFolder, fmt.Sprintf("clone-mount-%d-%d", mount.controller, mount.lun))
			if err != nil {
				return fmt.Errorf("error while creating directory for scsi mounts of clone vm: %s", err)
			}
		}

		// copy the VHDX
		dstVhdPath = filepath.Join(dir, filepath.Base(mount.hostPath))
		log.G(ctx).WithFields(logrus.Fields{
			"source hostPath":      mount.hostPath,
			"controller":           mount.controller,
			"LUN":                  mount.lun,
			"destination hostPath": dstVhdPath,
		}).Debug("Creating a clone of SCSI mount")

		if err = copyfile.CopyFile(ctx, mount.hostPath, dstVhdPath, true); err != nil {
			return err
		}

		if err = grantAccess(ctx, cd.UVMID, dstVhdPath, VMAccessTypeIndividual); err != nil {
			os.Remove(dstVhdPath)
			return err
		}
	}

	if cd.Doc.VirtualMachine.Devices.Scsi == nil {
		cd.Doc.VirtualMachine.Devices.Scsi = map[string]hcsschema.Scsi{}
	}

	if _, ok := cd.Doc.VirtualMachine.Devices.Scsi[conStr]; !ok {
		cd.Doc.VirtualMachine.Devices.Scsi[conStr] = hcsschema.Scsi{
			Attachments: map[string]hcsschema.Attachment{},
		}
	}

	cd.Doc.VirtualMachine.Devices.Scsi[conStr].Attachments[lunStr] = hcsschema.Attachment{
		Path:  dstVhdPath,
		Type_: string(mount.attachmentType),
	}

	r := m.NewMount(
		dstVhdPath,
		mount.guestPath,
		mount.attachmentType,
		mount.extensibleVirtualDiskType,
		1,
		mount.controller,
		mount.lun,
		mount.readOnly,
		mount.encrypted,
	)
	m.locations[mount.controller][mount.lun] = r
	return nil
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
		return fmt.Errorf("SCSI manager not initialized or already closed: %w", resource.ErrInvalidManagerState)
	}
	return nil
}

// must hold m.mu before calling.
func (m *Manager) invalid() bool { return m == nil || m.host == nil }

// AddRequest hold all the parameters that are sent to the Add method.
type AddRequest struct {
	// host path to the disk that should be added as a SCSI disk.
	HostPath string
	// the path inside the uvm at which this disk should show up. Can be empty.
	UVMPath string
	// AttachmentType is required and `must` be `VirtualDisk` for vhd/vhdx
	// attachments, `PassThru` for physical disk and `ExtensibleVirtualDisk` for
	// Extensible virtual disks.
	AttachmentType AttachmentType
	// indicates if the VHD is Encrypted
	Encrypted bool
	// indicates if the attachment should be added read only.
	ReadOnly bool
	// GuestOptions is a slice that contains optional information to pass to the guest
	// service.
	GuestOptions []string
	// indicates what access to grant the vm for the hostpath. Only required for
	// `VirtualDisk` and `PassThru` disk types.
	VMAccess VMAccessType
	// `EVDType` indicates the type of the extensible virtual disk if `attachmentType`
	// is "ExtensibleVirtualDisk" should be empty otherwise.
	EVDType string
}

type AttachmentType string

const (
	ExtensibleVirtualDiskAttachment AttachmentType = "ExtensibleVirtualDisk"
	PassThruAttachment              AttachmentType = "PassThru"
	VirtualDiskAttachment           AttachmentType = "VirtualDisk"
)

// VMAccessType is used to determine the various types of access we can
// grant for a given file.
type VMAccessType int

const (
	// `VMAccessTypeNoop` indicates no additional access should be given. Note
	// this should be used for layers and gpu vhd where we have given VM group
	// access outside of the shim (containerd for layers, package installation
	// for gpu vhd).
	VMAccessTypeNoop VMAccessType = iota
	// `VMAccessTypeGroup` indicates we should give access to a file for the VM group sid
	VMAccessTypeGroup
	// `VMAccessTypeIndividual` indicates we should give additional access to a file for
	// the running VM only
	VMAccessTypeIndividual
)

// grantAccess helper function to grant access to a file for the vm or vm group
func grantAccess(ctx context.Context, uvmID string, hostPath string, vmAccess VMAccessType) error {
	switch vmAccess {
	case VMAccessTypeGroup:
		log.G(ctx).WithField("path", hostPath).Debug("granting vm group access")
		return security.GrantVmGroupAccess(hostPath)
	case VMAccessTypeIndividual:
		return wclayer.GrantVmAccess(ctx, uvmID, hostPath)
	}
	return nil
}

// ParseExtensibleVirtualDiskPath parses the evd path provided in the config.
// extensible virtual disk path has format "evd://<evdType>/<evd-mount-path>"
// this function parses that and returns the `evdType` and `evd-mount-path`.
func ParseExtensibleVirtualDiskPath(hostPath string) (evdType, mountPath string, err error) {
	trimmedPath := strings.TrimPrefix(hostPath, "evd://")
	separatorIndex := strings.Index(trimmedPath, "/")
	if separatorIndex <= 0 {
		return "", "", errors.Errorf("invalid extensible vhd path: %s", hostPath)
	}
	return trimmedPath[:separatorIndex], trimmedPath[separatorIndex+1:], nil
}
