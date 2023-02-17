//go:build windows

package uvm

import (
	"context"
	"fmt"

	hcsschema "github.com/Microsoft/hcsshim/internal/hcs/schema2"
	"github.com/Microsoft/hcsshim/internal/uvm/resource"
	"github.com/Microsoft/hcsshim/internal/uvm/resource/vsmb"
)

// DefaultVSMBOptions returns the default VSMB options. If readOnly is specified,
// returns the default VSMB options for a readonly share.
func (uvm *UtilityVM) DefaultVSMBOptions(readOnly bool) *hcsschema.VirtualSmbShareOptions {
	return uvm.vsmb.DefaultOptions(readOnly)
}

func (uvm *UtilityVM) SetSaveableVSMBOptions(opts *hcsschema.VirtualSmbShareOptions, readOnly bool) {
	uvm.vsmb.SetSaveableOptions(opts, readOnly)
}

// AddVSMB adds a VSMB share to a Windows utility VM. Each VSMB share is ref-counted and
// only added if it isn't already. This is used for read-only layers, mapped directories
// to a container, and for mapped pipes.
func (uvm *UtilityVM) AddVSMB(ctx context.Context, hostPath string, options *hcsschema.VirtualSmbShareOptions) (*vsmb.Share, error) {
	if !uvm.isWindows() {
		return nil, fmt.Errorf("unsupported OS type %q: %w", uvm.operatingSystem, resource.ErrNotSupported)
	}

	return uvm.vsmb.Add(ctx, hostPath, options)
}

// RemoveVSMB removes a VSMB share from a utility VM. Each VSMB share is ref-counted
// and only actually removed when the ref-count drops to zero.
func (uvm *UtilityVM) RemoveVSMB(ctx context.Context, hostPath string, readOnly bool) error {
	if !uvm.isWindows() {
		return fmt.Errorf("unsupported OS type %q: %w", uvm.operatingSystem, resource.ErrNotSupported)
	}

	return uvm.vsmb.FindAndRemove(ctx, hostPath, readOnly)
}

// GetVSMBUvmPath returns the guest path of a VSMB mount.
func (uvm *UtilityVM) GetVSMBUvmPath(ctx context.Context, hostPath string, readOnly bool) (string, error) {
	return uvm.vsmb.GetUVMPath(ctx, hostPath, readOnly)
}
