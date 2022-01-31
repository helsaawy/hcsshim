package vhd

import (
	"context"
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/Microsoft/hcsshim/internal/vhd/ioctl"
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

// WalkVolumes walks through a mounted volume GUID strings of the form:
//   `\\?\Volume{xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx}\`
//
// Cancelling the context will cancel walking
func WalkVolumes(ctx context.Context, f func(context.Context, string) error) (err error) {
	buff := make([]uint16, VolumeGUIDStringLength)

	h, err := windows.FindFirstVolume(&buff[0], VolumeGUIDStringLength*2)
	if err != nil {
		return err
	}
	// todo: what to do about errors with closing the handle?
	defer windows.FindVolumeClose(h)

	for {
		s := windows.UTF16ToString(buff)
		if s == "" {
			return nil
		}

		if err = f(ctx, s); err != nil {
			return err
		}

		// allow f to first handle ctx cancellation, if it wants to
		if err = ctx.Err(); err != nil {
			return err
		}

		err = windows.FindNextVolume(h, &buff[0], VolumeGUIDStringLength*2)
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
func GetVolumePathNamesForVolumeName(ctx context.Context, vol string) (paths []string, err error) {
	if len(vol) < VolumeGUIDStringLength-1 || len(vol) > VolumeGUIDStringLength {
		return paths, fmt.Errorf("volume name is the wrong size, found: %d, expected: %d", len(vol), VolumeGUIDStringLength)
	}

	p, err := windows.UTF16PtrFromString(vol)
	if err != nil {
		return paths, fmt.Errorf("converting %q to byte pointer: %w", vol, err)
	}

	var buff []uint16
	sz := uint32(MaxPathLength)

	for i := 0; i < 3; i++ {
		buff = make([]uint16, sz)

		err = windows.GetVolumePathNamesForVolumeName(p, &buff[0], uint32(len(buff)*2), &sz)
		if errors.Is(err, windows.ERROR_MORE_DATA) {
			continue
		}
		break
	}
	if err != nil {
		return paths, fmt.Errorf("getting volume path names: %w", err)
	}

	if sz < 2 {
		return paths, nil
	}

	// buffer has two null terminals, one for the last string, and one for the entire array
	paths = UTF16ToStringArray(buff[:sz-1])
	return paths, err
}

func OpenVolumeReadOnly(ctx context.Context, vol string) (windows.Handle, error) {
	h := windows.InvalidHandle

	if vol[len(vol)-1] == '\\' {
		// is this utf-8 safe?
		vol = vol[:len(vol)-1]
	}

	p, err := windows.UTF16PtrFromString(vol)
	if err != nil {
		return h, fmt.Errorf("converting %q to byte pointer: %w", vol, err)
	}

	h, err = windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil /* sa */, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0 /*templatefile*/)
	if err != nil {
		return h, fmt.Errorf("opening volume: %w", err)
	}
	return h, nil
}

// https://docs.microsoft.com/en-us/windows/win32/api/winioctl/ni-winioctl-ioctl_volume_get_volume_disk_extents
func GetVolumeDeviceNumber(ctx context.Context, vol string) (uint32, error) {
	h, err := OpenVolumeReadOnly(ctx, vol)
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
