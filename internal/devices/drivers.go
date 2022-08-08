//go:build windows
// +build windows

package devices

import (
	"context"
	"fmt"

	"github.com/Microsoft/hcsshim/internal/guestpath"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/Microsoft/hcsshim/internal/resources"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/Microsoft/hcsshim/pkg/annotations/payload"
	"github.com/sirupsen/logrus"
)

// InstallKernelDriver mounts a specified kernel driver, then installs it in the UVM.
//
// `driver` is a directory path on the host that contains driver files for standard installation.
// For windows this means files for pnp installation (.inf, .cat, .sys, .cert files).
// For linux this means a vhd file that contains the drivers under /lib/modules/`uname -r` for use
// with depmod and modprobe.
//
// Returns a ResourceCloser for the added mount. On failure, the mounted share will be released,
// the returned ResourceCloser will be nil, and an error will be returned.
func InstallKernelDriver(ctx context.Context, vm *uvm.UtilityVM, driver *payload.Driver) (closers []resources.ResourceCloser, err error) {
	log.G(ctx).WithFields(logrus.Fields{
		logfields.UVMID: vm.ID(),
		"driver":        driver,
	}).Trace("Installing kernrel driver")

	defer func() {
		if err == nil {
			return
		}
		for _, closer := range closers {
			// best effort clean up allocated resource on failure
			if releaseErr := closer.Release(ctx); releaseErr != nil {
				log.G(ctx).WithError(releaseErr).Error("failed to release container resource")
			}
		}
		closers = nil
	}()

	if vm.OS() == "windows" {
		options := vm.DefaultVSMBOptions(true)
		closer, err := vm.AddVSMB(ctx, driver.Path, options)
		if closer != nil {
			closers = append(closers, closer)
		}
		if err != nil {
			return closers, fmt.Errorf("failed to add VSMB share to utility VM for path %+v: %s", driver, err)
		}
		uvmPath, err := vm.GetVSMBUvmPath(ctx, driver.Path, true)
		if err != nil {
			return closers, err
		}
		switch driver.Type {
		case payload.DriverTypeWindows:
			return closers, execPnPInstallDriver(ctx, vm, uvmPath)
		case payload.DriverTypeWindowsLegacy:
			closer, err := execDeviceUtilInstallDriver(ctx, vm, uvmPath, driver.Name)
			closers = append(closers, closer)
			return closers, err
		default:
		}
		return closers, fmt.Errorf("unknown driver type %v", driver.Type)
	}
	uvmPathForShare := fmt.Sprintf(guestpath.LCOWGlobalMountPrefixFmt, vm.UVMMountCounter())
	closer, err := vm.AddSCSI(ctx, driver.Path, uvmPathForShare, true, false, []string{}, uvm.VMAccessTypeIndividual)
	if closer != nil {
		closers = append(closers, closer)
	}
	if err != nil {
		return closers, fmt.Errorf("failed to add SCSI disk to utility VM for path %+v: %w", driver, err)
	}
	return closers, execModprobeInstallDriver(ctx, vm, uvmPathForShare)
}
