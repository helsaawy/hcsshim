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
	if uvm.operatingSystem != "windows" {
		return nil, resource.ErrNotSupported
	}

	return uvm.vsmb.AddShare(ctx, hostPath, options)
}

// RemoveVSMB removes a VSMB share from a utility VM. Each VSMB share is ref-counted
// and only actually removed when the ref-count drops to zero.
func (uvm *UtilityVM) RemoveVSMB(ctx context.Context, hostPath string, readOnly bool) error {
	if uvm.operatingSystem != "windows" {
		return resource.ErrNotSupported
	}

	share, err := uvm.vsmb.FindShare(hostPath, readOnly)
	if err != nil {
		return fmt.Errorf("%s is not present as a VSMB share in %s, cannot remove", hostPath, uvm.id)
	}

	return uvm.vsmb.Remove(ctx, share)
}

// GetVSMBUvmPath returns the guest path of a VSMB mount.
func (uvm *UtilityVM) GetVSMBUvmPath(_ context.Context, hostPath string, readOnly bool) (string, error) {
	return uvm.vsmb.GetUVMPath(hostPath, readOnly)
}
