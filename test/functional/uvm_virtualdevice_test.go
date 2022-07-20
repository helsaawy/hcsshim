//go:build functional
// +build functional

package functional

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	osversion "github.com/Microsoft/hcsshim/internal/os/version"
	"github.com/Microsoft/hcsshim/internal/uvm"
	testutilities "github.com/Microsoft/hcsshim/test/functional/utilities"
)

const lcowGPUBootFilesPath = "C:\\ContainerPlat\\LinuxBootFiles\\nvidiagpu"

// findTestDevices returns the first pcip device on the host
func findTestVirtualDevice() (string, error) {
	out, err := exec.Command(
		"powershell",
		`(Get-PnpDevice -presentOnly | where-object {$_.InstanceID -Match 'PCIP.*'})[0].InstanceId`,
	).Output()
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

func TestVirtualDevice(t *testing.T) {
	testutilities.RequiresBuild(t, osversion.V20H1)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	testDeviceInstanceID, err := findTestVirtualDevice()
	if err != nil {
		t.Skipf("skipping test, failed to find assignable device on host with: %v", err)
	}
	if testDeviceInstanceID == "" {
		t.Skipf("skipping test, host has no assignable PCIP devices")
	}

	// update opts needed to assign a hyper-v pci device
	opts := uvm.NewDefaultOptionsLCOW(t.Name(), "")
	opts.VPCIEnabled = true
	opts.AllowOvercommit = false
	opts.KernelDirect = false
	opts.VPMemDeviceCount = 0
	opts.KernelFile = uvm.KernelFile
	opts.RootFSFile = uvm.InitrdFile
	opts.PreferredRootFSType = uvm.PreferredRootFSTypeInitRd
	opts.BootFilesPath = lcowGPUBootFilesPath

	// create test uvm and ensure we can assign and remove the device
	vm := testutilities.CreateLCOWUVMFromOpts(ctx, t, opts)
	defer vm.Close()
	vpci, err := vm.AssignDevice(ctx, testDeviceInstanceID, 0, "")
	if err != nil {
		t.Fatalf("failed to assign device %s with %v", testDeviceInstanceID, err)
	}
	if err := vpci.Release(ctx); err != nil {
		t.Fatalf("failed to remove device %s with %v", testDeviceInstanceID, err)
	}
}
