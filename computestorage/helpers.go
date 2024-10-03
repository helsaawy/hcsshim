//go:build windows

package computestorage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Microsoft/go-winio/vhd"
	"github.com/Microsoft/hcsshim/internal/memory"
	"golang.org/x/sys/windows"

	"github.com/Microsoft/hcsshim/internal/security"
)

const (
	defaultVHDXBlockSizeInMB = 1
)

// SetupContainerBaseLayer is a helper to setup a containers scratch. It
// will create and format the vhdx's inside and the size is configurable with the sizeInGB
// parameter.
//
// `layerPath` is the path to the base container layer on disk.
//
// `baseVHDPath` is the path to where the base vhdx for the base layer should be created.
//
// `diffVHDPath` is the path where the differencing disk for the base layer should be created.
//
// `sizeInGB` is the size in gigabytes to make the base vhdx.
func SetupContainerBaseLayer(ctx context.Context, layerPath, baseVHDPath, diffVHDPath string, sizeInGB uint64) (err error) {
	// We need to remove the hives directory and layout file as `SetupBaseOSLayer` fails if these files
	// already exist. `SetupBaseOSLayer` will create these files internally. We also remove the base and
	// differencing disks if they exist in case we're asking for a different size.
	if err := removeLayerState(layerPath, baseVHDPath, diffVHDPath); err != nil {
		return err
	}

	createParams := &vhd.CreateVirtualDiskParameters{
		Version: 2,
		Version2: vhd.CreateVersion2{
			MaximumSize:      sizeInGB * memory.GiB,
			BlockSizeInBytes: defaultVHDXBlockSizeInMB * memory.MiB,
		},
	}
	handle, err := vhd.CreateVirtualDisk(baseVHDPath, vhd.VirtualDiskAccessNone, vhd.CreateVirtualDiskFlagNone, createParams)
	if err != nil {
		return fmt.Errorf("failed to create vhdx: %w", err)
	}

	defer func() {
		if err != nil {
			_ = syscall.CloseHandle(handle)
			os.RemoveAll(baseVHDPath)
			os.RemoveAll(diffVHDPath)
		}
	}()

	if err = FormatWritableLayerVhd(ctx, windows.Handle(handle)); err != nil {
		return err
	}
	// Base vhd handle must be closed before calling SetupBaseLayer in case of Container layer
	if err = syscall.CloseHandle(handle); err != nil {
		return fmt.Errorf("failed to close vhdx handle: %w", err)
	}

	options := OsLayerOptions{
		Type: OsLayerTypeContainer,
	}

	// SetupBaseOSLayer expects an empty vhd handle for a container layer and will
	// error out otherwise.
	if err = SetupBaseOSLayer(ctx, layerPath, 0, options); err != nil {
		return err
	}
	// Create the differencing disk that will be what's copied for the final rw layer
	// for a container.
	if err = vhd.CreateDiffVhd(diffVHDPath, baseVHDPath, defaultVHDXBlockSizeInMB); err != nil {
		return fmt.Errorf("failed to create differencing disk: %w", err)
	}

	if err = security.GrantVmGroupAccess(baseVHDPath); err != nil {
		return fmt.Errorf("failed to grant vm group access to %q: %w", baseVHDPath, err)
	}
	if err = security.GrantVmGroupAccess(diffVHDPath); err != nil {
		return fmt.Errorf("failed to grant vm group access to %s: %w", diffVHDPath, err)
	}
	return nil
}

// SetupUtilityVMBaseLayer is a helper to setup a UVMs scratch space. It will create and format
// the vhdx inside and the size is configurable by the sizeInGB parameter.
//
// `uvmPath` is the path to the UtilityVM filesystem.
//
// `baseVHDPath` is the path to where the base vhdx for the UVM should be created.
//
// `diffVHDPath` is the path where the differencing disk for the UVM should be created.
//
// `sizeInGB` specifies the size in gigabytes to make the base vhdx.
func SetupUtilityVMBaseLayer(ctx context.Context, uvmPath, baseVHDPath, diffVHDPath string, sizeInGB uint64) (err error) {
	// Remove the base and differencing disks if they exist in case we're asking for a different size.
	// This will also attempt to remove `uvmPath/Hives/` and `uvmPath/Layout`, which shouldn't exist, but need to be removed
	// if they do.
	if err := removeLayerState(uvmPath, baseVHDPath, diffVHDPath); err != nil {
		return err
	}

	// Just create the vhdx for utilityVM layer, no need to format it.
	createParams := &vhd.CreateVirtualDiskParameters{
		Version: 2,
		Version2: vhd.CreateVersion2{
			MaximumSize:      sizeInGB * memory.GiB,
			BlockSizeInBytes: defaultVHDXBlockSizeInMB * memory.MiB,
		},
	}
	handle, err := vhd.CreateVirtualDisk(baseVHDPath, vhd.VirtualDiskAccessNone, vhd.CreateVirtualDiskFlagNone, createParams)
	if err != nil {
		return fmt.Errorf("failed to create vhdx: %w", err)
	}

	defer func() {
		if err != nil {
			_ = syscall.CloseHandle(handle)
			os.RemoveAll(baseVHDPath)
			os.RemoveAll(diffVHDPath)
		}
	}()

	// If it is a UtilityVM layer then the base vhdx must be attached when calling
	// `SetupBaseOSLayer`
	attachParams := &vhd.AttachVirtualDiskParameters{
		Version: 2,
	}
	if err := vhd.AttachVirtualDisk(handle, vhd.AttachVirtualDiskFlagNone, attachParams); err != nil {
		return fmt.Errorf("failed to attach virtual disk: %w", err)
	}

	options := OsLayerOptions{
		Type: OsLayerTypeVM,
	}
	if err := SetupBaseOSLayer(ctx, uvmPath, windows.Handle(handle), options); err != nil {
		return err
	}

	// Detach and close the handle after setting up the layer as we don't need the handle
	// for anything else and we no longer need to be attached either.
	if err = vhd.DetachVirtualDisk(handle); err != nil {
		return fmt.Errorf("failed to detach vhdx: %w", err)
	}
	if err = syscall.CloseHandle(handle); err != nil {
		return fmt.Errorf("failed to close vhdx handle: %w", err)
	}

	// Create the differencing disk that will be what's copied for the final rw layer
	// for a container.
	if err = vhd.CreateDiffVhd(diffVHDPath, baseVHDPath, defaultVHDXBlockSizeInMB); err != nil {
		return fmt.Errorf("failed to create differencing disk: %w", err)
	}

	if err := security.GrantVmGroupAccess(baseVHDPath); err != nil {
		return fmt.Errorf("failed to grant vm group access to %q: %w", baseVHDPath, err)
	}
	if err := security.GrantVmGroupAccess(diffVHDPath); err != nil {
		return fmt.Errorf("failed to grant vm group access to %q: %w", diffVHDPath, err)
	}
	return nil
}

// removeLayerState removes the base and differencing VHDx's, as well as the Hives directory
// and Layout file (both under layerPath), if they exist.
func removeLayerState(layerPath, baseVHD, diffVHD string) error {
	var errs []error
	for _, x := range []struct {
		name string
		path string
	}{
		{
			name: "hives directory",
			path: filepath.Join(layerPath, "Hives"),
		},
		{
			name: "layout file",
			path: filepath.Join(layerPath, "Layout"),
		},
		{
			name: "base vhdx",
			path: baseVHD,
		},
		{
			name: "differencing vhdx",
			path: diffVHD,
		},
	} {
		if err := os.RemoveAll(x.path); err != nil {
			errs = append(errs, fmt.Errorf("remove %s (%q): %w", x.name, x.path, err))
		}
	}

	return errors.Join(errs...)
}
