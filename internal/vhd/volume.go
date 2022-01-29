package vhd

import (
	"errors"
	"fmt"
	"regexp"
	"syscall"
	"unsafe"

	"github.com/Microsoft/hcsshim/internal/vhd/ioctl"
	"golang.org/x/sys/windows"
)

const (
	// MaxPath is the maximum length for a path. A local path is structured in the following
	// order: drive letter, colon, backslash, name components separated by backslashes,
	// and a terminating null character.
	//
	// see also: https://docs.microsoft.com/en-us/windows/win32/fileio/maximum-file-path-limitation
	MaxPath = 260

	// VolumeGUIDStringLength is the length of a null-terminated Volume GUID string of the form:
	//   \\?\Volume{xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx}\
	VolumeGUIDStringLength = 50

	hexCG = `[\da-f]`
)

var (
	DiskNumberRegex = regexp.MustCompile(`\\\\.\\PhysicalDrive([\d]+)`)
	VolumeGUIDRegex = regexp.MustCompile(
		`(?i)\\\\\?\\Volume{(` +
			hexCG + `{8}-` +
			hexCG + `{4}-` +
			hexCG + `{4}-` +
			hexCG + `{4}-` +
			hexCG + `{12}` + `)}\\?`)
)

type VolumeGUID struct {
	// todo, add ParseString() function for VolumeGUID
	g GUID
}

func (v VolumeGUID) GUID() GUID {
	return v.g
}

// most things need the training \
func (v VolumeGUID) String() string {
	return `\\?\Volume{` + v.g.String() + `}\`
}

// todo: add version where buffer is parsed as a VolumeGUID and passed to func

func bytesToString(b []byte) string {
	var i int
	for i < len(b) && b[i] != 0 {
		i++
	}
	return string(b[:i])
}

func WalkVolumesA(f func(string) error) (err error) {
	buff := make([]byte, VolumeGUIDStringLength)

	h, err := findFirstVolumeA(&buff[0], VolumeGUIDStringLength)
	if err != nil {
		return err
	}
	// todo: what to do about errors with closing the handle?
	defer findVolumeClose(h)

	for {
		if err = f(bytesToString(buff[:])); err != nil {
			return err
		}

		err = findNextVolumeA(h, &buff[0], VolumeGUIDStringLength)
		if err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				err = nil
			}
			return err
		}
	}
}

// GetVolumePathNamesForVolumeName retrieves a list of drive letters and mounted
// folder paths for the specified volume.
// vol must be a properly formatted volume GUID string
func GetVolumePathNamesForVolumeName(vol string) (paths []string, err error) {
	if len(vol) < VolumeGUIDStringLength-1 || len(vol) > VolumeGUIDStringLength {
		return paths, fmt.Errorf("volume name is the wrong size, found: %d, expected: %d", len(vol), VolumeGUIDStringLength)
	}

	v, err := syscall.BytePtrFromString(vol)
	if err != nil {
		return paths, fmt.Errorf("converting %q to byte pointer: %w", vol, err)
	}

	var l uint32
	buff := make([]byte, 256)
	for {
		err = getVolumePathNamesForVolumeNameA(v, &buff[0], uint32(len(buff)), &l)
		if errors.Is(err, windows.ERROR_MORE_DATA) {
			buff = make([]byte, l)
		} else if err != nil {
			return paths, fmt.Errorf("getting volume path names: %w", err)
		} else {
			break
		}
	}

	if l < 2 {
		return paths, nil
	}

	// buffer has two null terminals, one for the last string, and one for the entire array
	for i := uint32(0); i < l-1; {
		j := i
		for buff[j] != 0 && j < l {
			j++
		}
		paths = append(paths, string(buff[i:j+1]))
		i = j + 1
	}
	return paths, err
}

func OpenVolumeReadOnly(vol string) (windows.Handle, error) {
	h := windows.InvalidHandle

	if vol[len(vol)-1] == '\\' {
		// is this utf-8 safe?
		vol = vol[:len(vol)-1]
	}

	b, err := syscall.BytePtrFromString(vol)
	if err != nil {
		return h, fmt.Errorf("converting %q to byte pointer: %w", vol, err)
	}

	h, err = CreateFileA(b, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil /* sa */, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0 /*templatefile*/)
	if err != nil {
		return h, fmt.Errorf("opening volume: %w", err)
	}
	return h, nil
}

// https://docs.microsoft.com/en-us/windows/win32/api/winioctl/ni-winioctl-ioctl_volume_get_volume_disk_extents
func GetVolumeDeviceNumber(vol string) (uint32, error) {
	h, err := OpenVolumeReadOnly(vol)
	if err != nil {
		return 0, fmt.Errorf("volume device number: %w", err)
	}
	defer windows.CloseHandle(h)

	s := unsafe.Sizeof(ioctl.VolumeDiskExtents{})
	b := make([]byte, s)
	var r uint32

	// todo: handle ERROR_INSUFFICIENT_BUFFER and ERROR_MORE_DATA if buffer is too small
	err = windows.DeviceIoControl(h, uint32(ioctl.IOCTL_VOLUME_GET_VOLUME_DISK_EXTENTS),
		nil /*inBuffer*/, 0, /*inBufferSize*/
		&b[0], uint32(s), &r, nil /*overlapped*/)
	if err != nil {
		return 0, fmt.Errorf("device IO control get volume disk extents API call: %w", err)
	}
	var v = (*ioctl.VolumeDiskExtents)(unsafe.Pointer(&b[0]))

	return v.DiskExtents[0].DiskNumber, nil
}
