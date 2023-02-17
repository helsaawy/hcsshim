//go:build windows

package uvm

import (
	"context"
	"fmt"

	"github.com/Microsoft/hcsshim/internal/uvm/resource"
	"github.com/Microsoft/hcsshim/internal/uvm/resource/plan9"
)

// AddPlan9 adds a Plan9 share to a utility VM.
func (uvm *UtilityVM) AddPlan9(ctx context.Context, hostPath string, uvmPath string, readOnly bool, restrict bool, allowedNames []string) (*plan9.Share, error) {
	if uvm.operatingSystem != "linux" {
		return nil, fmt.Errorf("unsupported OS type %q: %w", uvm.operatingSystem, resource.ErrNotSupported)
	}

	return uvm.plan9.Add(ctx, hostPath, uvmPath, readOnly, restrict, allowedNames)
}

// RemovePlan9 removes a Plan9 share from a utility VM.
func (uvm *UtilityVM) RemovePlan9(ctx context.Context, share *plan9.Share) error {
	if uvm.operatingSystem != "linux" {
		return fmt.Errorf("unsupported OS type %q: %w", uvm.operatingSystem, resource.ErrNotSupported)
	}

	return uvm.plan9.Remove(ctx, share)
}
