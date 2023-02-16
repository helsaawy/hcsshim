//go:build windows

package uvm

import (
	"context"
)

func (uvm *UtilityVM) Pause(ctx context.Context) error {
	return uvm.hcsSystem.Pause(ctx)
}

func (uvm *UtilityVM) Resume(ctx context.Context) error {
	return uvm.hcsSystem.Resume(ctx)
}

func (uvm *UtilityVM) Stop(ctx context.Context) error {
	return uvm.hcsSystem.Shutdown(ctx)
}