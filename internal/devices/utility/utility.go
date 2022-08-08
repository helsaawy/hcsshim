package utility

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Microsoft/hcsshim/internal/resources"
	"github.com/Microsoft/hcsshim/internal/uvm"
)

const deviceUtilExeName = "device-util.exe"

// getDeviceUtilHostPath is a simple helper function to find the host path of the device-util tool
func getDeviceUtilHostPath() string {
	// todo: switch to using `os.Executable` instead?
	return filepath.Join(filepath.Dir(os.Args[0]), deviceUtilExeName)
}

// InstallDeviceUtility shares the `device-util` tool into the guest, and returns the guest path as well
// as a closer for the share
//
// The returned resource closer is always non-nil.
func InstallDeviceUtility(ctx context.Context, vm *uvm.UtilityVM) (string, resources.ResourceCloser, error) {
	// install the device util tool in the UVM
	hostPath := getDeviceUtilHostPath()
	share, err := vm.AddVSMB(ctx, hostPath, vm.DefaultVSMBOptions(true))
	if err != nil {
		return "", resources.NopResourceCloser(), fmt.Errorf("failed to add vSMB share to uVM for path %q: %w", hostPath, err)
	}
	guestPath, err := vm.GetVSMBUvmPath(ctx, hostPath, true)
	if err != nil {
		return "", share, fmt.Errorf("failed to get utility path in uVM for share %q: %w", hostPath, err)
	}

	return guestPath, share, nil
}

// CreateChildrenCommand constructs a device-util command to query the UVM for
// device information
//
// `deviceUtilPath` is the UVM path to device-util
//
// `vmBusInstanceID` is a string of the vmbus instance ID already assigned to the UVM
//
// Returns a slice of strings that represent the location paths in the UVM of the
// target devices
func CreateChildrenCommand(deviceUtilPath, vmBusInstanceID string) []string {
	parentIDsFlag := fmt.Sprintf("--parentID=%s", vmBusInstanceID)
	args := []string{deviceUtilPath, "children", parentIDsFlag, "--property=location"}
	return args
}

// CreateInstallDriverCommand constructs the command args for the device-util command to
// install the legacy driver at path inside the uVM.
// deviceUtilPath is the uVM path that device-util is mounted at.
//
// If provided, the driver's display name will be name.
func CreateInstallDriverCommand(deviceUtilPath, path, name string) []string {
	// Currently, the utility and drivers are shared in as raw vSMB (file) shares, and not mapped to
	// a guest path. however, cmd prompt does not handle UNC paths, so these commands will fail;
	//	cmd /c copy \\?\VMSMB\VSMB-{dcc079ae-60ba-4d07-847c-3493609c0870}\s1\driver.sys %SystemRoot%\System32\drivers\
	//  cmd /c \\?\VMSMB\VSMB-{dcc079ae-60ba-4d07-847c-3493609c0870}\s2\device-utility kd i -p driver.sys
	//
	// However, using the UNC path directory as the command works fine.
	// So, until drivers get mapped somewhere on disk, do not bother trying to copy them into %SystemRoot%\System32\drivers
	args := []string{deviceUtilPath, "kernel-driver", "install", "-wait", "-path", path}
	if name != "" {
		args = append(args, "-name", `"`+name+`"`)
	}
	return args
}
