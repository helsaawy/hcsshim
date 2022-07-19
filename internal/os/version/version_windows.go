//go:build windows

package version

import (
	"fmt"
	"sync"

	"golang.org/x/sys/windows"
)

// Version is a wrapper for Windows version information
// https://msdn.microsoft.com/en-us/library/windows/desktop/ms724439(v=vs.85).aspx
type Version struct {
	Version      uint32
	MajorVersion uint8
	MinorVersion uint8
	Build        uint16
}

var (
	osv  Version
	once sync.Once
)

// Get gets the operating system version on Windows.
// The calling application must be manifested to get the correct version information.
func Get() Version {
	once.Do(func() {
		var err error
		osv = Version{}
		osv.Version, err = windows.GetVersion()
		if err != nil {
			// GetVersion never fails.
			panic(err)
		}
		osv.MajorVersion = uint8(osv.Version & 0xFF)
		osv.MinorVersion = uint8(osv.Version >> 8 & 0xFF)
		osv.Build = uint16(osv.Version >> 16)
	})
	return osv
}

// Build gets the build-number on Windows
// The calling application must be manifested to get the correct version information.
func Build() uint16 {
	return Get().Build
}

func (osv Version) ToString() string {
	return fmt.Sprintf("%d.%d.%d", osv.MajorVersion, osv.MinorVersion, osv.Build)
}

// implement fmt.Stringer

func (osv Version) String() string {
	return osv.ToString()
}
