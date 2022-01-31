package vhd

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/Microsoft/go-winio/pkg/guid"
	"golang.org/x/sys/windows"
)

// miscellaneous byte and string manipulation

const (
	// MaxPathLength is the maximum length for a path. A local path is structured in
	// the following order: drive letter, colon, backslash, name components separated
	// by backslashes, and a terminating null character.
	//
	// see also: https://docs.microsoft.com/en-us/windows/win32/fileio/maximum-file-path-limitation
	MaxPathLength = 260

	// VolumeGUIDStringLength is the length of a null-terminated Volume GUID string of the form:
	//   \\?\Volume{xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx}\
	VolumeGUIDStringLength = 50

	hexCG = `[\da-f]`
)

var (
	DiskNumberRegex = regexp.MustCompile(`(?i)\\\\.\\PhysicalDrive([\d]+)`)
	VolumeGUIDRegex = regexp.MustCompile(
		`(?i)\\\\\?\\Volume{(` +
			hexCG + `{8}-` +
			hexCG + `{4}-` +
			hexCG + `{4}-` +
			hexCG + `{4}-` +
			hexCG + `{12}` + `)}\\?`)
)

// ParseDiskNumber extracts the disk number from the physical device path of the form
//    `\\.\PhysicalDriveX`
// Returns a `strconv.ErrSyntax` if the string is not of the correct form.
//
// see GetVirtualDiskPhysicalPath
func ParseDiskNumber(s string) (n uint32, err error) {
	m := DiskNumberRegex.FindStringSubmatch(s)
	if len(m) != 2 {
		return n, fmt.Errorf("%q does not match regex %q: %w", s, DiskNumberRegex.String(), strconv.ErrSyntax)
	}

	// disk number is a DWORD (32 bit integer):
	// see https://docs.microsoft.com/en-us/windows/win32/api/winioctl/ns-winioctl-disk_extent
	nn, err := strconv.ParseUint(m[1], 10, 32)
	if err != nil {
		return n, fmt.Errorf("could not parse disk number %q: %w", m[1], err)
	}

	return uint32(nn), nil
}

// ParseVolumeGUID extacts the GUID from a volume string name of the form
//   \\?\Volume{xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx}\
// Returns a `strconv.ErrSyntax` if the string is not of the correct form.
//
// see FindFirstVolume, GetVolumePathNamesForVolumeName
func ParseVolumeGUID(s string) (g GUID, err error) {
	m := VolumeGUIDRegex.FindStringSubmatch(s)
	if len(m) != 2 {
		return g, fmt.Errorf("%q does not match regex %q: %w", s, DiskNumberRegex.String(), strconv.ErrSyntax)
	}

	g, err = guid.FromString(m[1])
	if err != nil {
		err = fmt.Errorf("%s: %w", err, strconv.ErrSyntax)
	}

	return g, nil
}

func bytesToString(s []byte) string {
	for i, v := range s {
		if v == 0 {
			s = s[:i]
			break
		}
	}
	return string(s)
}

func bytesToStringArray(s []byte) []string {
	a := make([]string, 0, 3)

	for i := 0; i < len(s); {
		ss := bytesToString(s[i:])
		l := len(ss)
		if l > 0 {
			// skip empty strings (ie, repeated null bytes)
			a = append(a, ss)
		}
		i += l + 1
	}
	return a
}

func UTF16ToStringArray(s []uint16) []string {
	a := make([]string, 0, 3)

	// cant modify the `i` in `i,v := range s`, so need old-school for loop to
	// modify index and skip ahead after processing a string
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			continue
		}

		// don't modify s inside the loop
		si := s[i:]
		ss := windows.UTF16ToString(si)
		if len(s) > 0 { // skip empty strings (ie, failed parsings)
			a = append(a, ss)
		}

		// len(s) returns the bytes needed to encode s as a utf-8 string, not utf-16
		for j, v := range si {
			if v == 0 {
				i += j + 1
				break
			}
		}
	}
	return a
}
