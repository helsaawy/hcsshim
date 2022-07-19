//go:build windows

package osversion

import (
	"github.com/Microsoft/hcsshim/internal/os/version"
)

// OSVersion is a wrapper for Windows version information
// https://msdn.microsoft.com/en-us/library/windows/desktop/ms724439(v=vs.85).aspx
type OSVersion = version.Version

// Get gets the operating system version on Windows.
// The calling application must be manifested to get the correct version information.
func Get() OSVersion {
	return version.Get()
}

// Build gets the build-number on Windows
// The calling application must be manifested to get the correct version information.
func Build() uint16 {
	return version.Build()
}
