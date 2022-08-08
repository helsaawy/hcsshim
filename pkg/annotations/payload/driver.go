package payload

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

//go:generate go run golang.org/x/tools/cmd/stringer -type=DriverType -linecomment

// Driver defines options and customizations used when installing drivers inside of a utility VM
// with the [VirtualMachineKernelDrivers] annotation.
type Driver struct {
	// Path is the location of the drivers.
	// This is a directory for Windows, a *.sys file for WindowsLegacy, or a VHD for Linux.
	Path string `json:"path,omitempty"`
	// Type specifies the type of driver and how it is to be installed.
	// Currently this only affects Windows drivers, and selects between installing via pnputils
	// or creating a kernel-driver type service.
	Type DriverType `json:"type,omitempty"`
	// Name is the display name of the driver. This only affects WindowsLegacy type drivers.
	Name string `json:"name,omitempty"`
}

// CreateDriversAnnotationPayload generates the string payload to to use with the
// [VirtualMachineKernelDrivers] annotation.
func CreateDriverAnnotationPayload(drivers ...Driver) (string, error) {
	b, err := json.Marshal(drivers)
	return string(b), err
}

// Validate checks that the [Driver] fields are appropriate for its [DriverType].
func (d *Driver) Validate() error {
	fi, err := os.Stat(d.Path)
	if err != nil {
		return fmt.Errorf("invalid driver path %q: %w", d.Path, err)
	}

	errPath := fmt.Errorf("invalid path for driver of type %v", d.Type)
	switch d.Type {
	case DriverTypeWindows:
		if !fi.IsDir() {
			return fmt.Errorf("%q is not a directory: %w", d.Path, errPath)
		}
	case DriverTypeWindowsLegacy:
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("%q is not a file: %w", d.Path, errPath)
		}
	case DriverTypeLinux:
		ext := filepath.Ext(d.Path)
		if !(fi.Mode().IsRegular() && (ext == ".vhd" || ext == ".vhdx")) {
			return fmt.Errorf("%q is not a VHD file: %w", d.Path, errPath)
		}
	default:
		return fmt.Errorf("unknown DriverType %v", d.Type)
	}

	return nil
}

type DriverType int32

const (
	// PnP drivers installed via `pnputil.exe /add-driver /install`.
	DriverTypeWindows DriverType = iota // Windows
	// Legacy drivers installed by `CreateService` and `StartService` syscalls.
	// The created service will have type `SERVICE_KERNEL_DRIVER`.
	DriverTypeWindowsLegacy // WindowsLegacy
	// Linux kernel modules installed with `depmod`` and `modprobe`.
	DriverTypeLinux // Linux
)

// driverTypeValues pre-computes the translation for string to driverType.
// Used to for JSON unmarshal lookup, and for testing.
var driverTypeValues = map[string]DriverType{}

func init() {
	for i := 0; i < len(_DriverType_index)-1; i++ {
		dt := DriverType(i)
		driverTypeValues[dt.String()] = dt
	}
}

// have driver type (un)marshal as string instead of int for JSON

func (dt DriverType) MarshalJSON() ([]byte, error) {
	return json.Marshal(dt.String())
}

func (dt *DriverType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	if v, ok := driverTypeValues[s]; ok {
		*dt = v
		return nil
	}
	return fmt.Errorf("unknown DriverType %q", dt.String())
}
