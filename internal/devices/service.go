//go:build windows
// +build windows

package devices

import (
	"context"
	"fmt"

	"github.com/Microsoft/hcsshim/internal/cmd"
	"github.com/Microsoft/hcsshim/internal/devices/utility"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/Microsoft/hcsshim/internal/resources"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/sirupsen/logrus"
)

// execDeviceUtilInstallDriver makes the calls to exec device-util
// to install the driver previously mounted into the uVM, and setting its name if one is provided
//
// The returned resource closer is always non-nil.
func execDeviceUtilInstallDriver(ctx context.Context, vm *uvm.UtilityVM, path, name string) (resources.ResourceCloser, error) {
	utilPath, closer, err := utility.InstallDeviceUtility(ctx, vm)
	if err != nil {
		return closer, err
	}

	args := utility.CreateInstallDriverCommand(utilPath, path, name)
	c := cmd.CommandContext(ctx, vm, args[0], args[1:]...)
	c.Spec.User.Username = `NT AUTHORITY\SYSTEM`
	c.Log = log.G(ctx).WithField(logfields.UVMID, vm.ID())
	out, errb, err := c.Outputs()
	entry := log.G(ctx).WithFields(logrus.Fields{
		"driver": path,
		"name":   name,
		"out":    string(out),
		"err":    string(errb),
		"cmd":    c.Spec.Args,
	})
	if err != nil {
		entry.WithError(err).Error("Driver installation failed")
		return closer, fmt.Errorf("driver install for %q failed: %w", path, err)
	}
	entry.Error("legacy driver install finished")
	return closer, nil
}
