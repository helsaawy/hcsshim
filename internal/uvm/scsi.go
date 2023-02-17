//go:build windows

package uvm

import (
	"context"

	"github.com/Microsoft/hcsshim/internal/uvm/resource/scsi"
)

// RemoveSCSI removes a SCSI disk from a utility VM.
func (uvm *UtilityVM) RemoveSCSI(ctx context.Context, hostPath string) error {
	return uvm.scsi.FindAndRemove(ctx, hostPath)
}

// AddSCSI adds a SCSI disk to a utility VM at the next available location. This
// function should be called for adding a scratch layer, a read-only layer as an
// alternative to VPMEM, or for other VHD mounts.
//
// `hostPath` is required and must point to a vhd/vhdx path.
//
// `uvmPath` is optional. If not provided, no guest request will be made
//
// `readOnly` set to `true` if the vhd/vhdx should be attached read only.
//
// `encrypted` set to `true` if the vhd/vhdx should be attached in encrypted mode.
// The device will be formatted, so this option must be used only when creating
// scratch vhd/vhdx.
//
// `guestOptions` is a slice that contains optional information to pass
// to the guest service
//
// `vmAccess` indicates what access to grant the vm for the hostpath
func (uvm *UtilityVM) AddSCSI(
	ctx context.Context,
	hostPath string,
	uvmPath string,
	readOnly bool,
	encrypted bool,
	guestOptions []string,
	vmAccess scsi.VMAccessType,
) (*scsi.Mount, error) {
	addReq := &scsi.AddRequest{
		HostPath:       hostPath,
		UVMPath:        uvmPath,
		AttachmentType: scsi.VirtualDiskAttachment,
		ReadOnly:       readOnly,
		Encrypted:      encrypted,
		GuestOptions:   guestOptions,
		VMAccess:       vmAccess,
	}
	return uvm.scsi.Add(ctx, addReq)
}

// AddSCSIPhysicalDisk attaches a physical disk from the host directly to the
// Utility VM at the next available location.
//
// `hostPath` is required and `likely` start's with `\\.\PHYSICALDRIVE`.
//
// `uvmPath` is optional if a guest mount is not requested.
//
// `readOnly` set to `true` if the physical disk should be attached read only.
//
// `guestOptions` is a slice that contains optional information to pass
// to the guest service
func (uvm *UtilityVM) AddSCSIPhysicalDisk(ctx context.Context, hostPath, uvmPath string, readOnly bool, guestOptions []string) (*scsi.Mount, error) {
	addReq := &scsi.AddRequest{
		HostPath:       hostPath,
		UVMPath:        uvmPath,
		AttachmentType: scsi.PassThruAttachment,
		ReadOnly:       readOnly,
		GuestOptions:   guestOptions,
		VMAccess:       scsi.VMAccessTypeIndividual,
	}
	return uvm.scsi.Add(ctx, addReq)
}

// AddSCSIExtensibleVirtualDisk adds an extensible virtual disk as a SCSI mount
// to the utility VM at the next available location. All such disks which are not actual virtual disks
// but provide the same SCSI interface are added to the UVM as Extensible Virtual disks.
//
// `hostPath` is required. Depending on the type of the extensible virtual disk the format of `hostPath` can
// be different.
// For example, in case of storage spaces the host path must be in the
// `evd://space/{storage_pool_unique_ID}{virtual_disk_unique_ID}` format.
//
// `uvmPath` must be provided in order to be able to use this disk in a container.
//
// `readOnly` set to `true` if the virtual disk should be attached read only.
//
// `vmAccess` indicates what access to grant the vm for the hostpath
func (uvm *UtilityVM) AddSCSIExtensibleVirtualDisk(ctx context.Context, hostPath, uvmPath string, readOnly bool) (*scsi.Mount, error) {
	evdType, mountPath, err := scsi.ParseExtensibleVirtualDiskPath(hostPath)
	if err != nil {
		return nil, err
	}
	addReq := &scsi.AddRequest{
		HostPath:       mountPath,
		UVMPath:        uvmPath,
		AttachmentType: scsi.ExtensibleVirtualDiskAttachment,
		ReadOnly:       readOnly,
		GuestOptions:   []string{},
		VMAccess:       scsi.VMAccessTypeIndividual,
		EVDType:        evdType,
	}
	return uvm.scsi.Add(ctx, addReq)
}

// GetScsiUvmPath returns the guest mounted path of a SCSI drive.
//
// If `hostPath` is not mounted returns `ErrNotAttached`.
func (uvm *UtilityVM) GetScsiUvmPath(ctx context.Context, hostPath string) (string, error) {
	return uvm.scsi.GetUVMPath(ctx, hostPath)
}
