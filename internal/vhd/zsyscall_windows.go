// Code generated mksyscall_windows.exe DO NOT EDIT

package vhd

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var _ unsafe.Pointer

// Do the interface allocations only once for common
// Errno values.
const (
	errnoERROR_IO_PENDING = 997
)

var (
	errERROR_IO_PENDING error = syscall.Errno(errnoERROR_IO_PENDING)
)

// errnoErr returns common boxed Errno values, to prevent
// allocations at runtime.
func errnoErr(e syscall.Errno) error {
	switch e {
	case 0:
		return nil
	case errnoERROR_IO_PENDING:
		return errERROR_IO_PENDING
	}
	// TODO: add more here, after collecting data on the common
	// error values see on Windows. (perhaps when running
	// all.bat?)
	return e
}

var (
	modkernel32 = windows.NewLazySystemDLL("kernel32.dll")
	modvirtdisk = windows.NewLazySystemDLL("virtdisk.dll")

	procCreateFileA                      = modkernel32.NewProc("CreateFileA")
	procFindFirstVolumeA                 = modkernel32.NewProc("FindFirstVolumeA")
	procFindNextVolumeA                  = modkernel32.NewProc("FindNextVolumeA")
	procFindVolumeClose                  = modkernel32.NewProc("FindVolumeClose")
	procFindFirstVolumeMountPointA       = modkernel32.NewProc("FindFirstVolumeMountPointA")
	procFindNextVolumeMountPointA        = modkernel32.NewProc("FindNextVolumeMountPointA")
	procFindVolumeMountPointClose        = modkernel32.NewProc("FindVolumeMountPointClose")
	procGetVolumePathNamesForVolumeNameA = modkernel32.NewProc("GetVolumePathNamesForVolumeNameA")
	procOpenVirtualDisk                  = modvirtdisk.NewProc("OpenVirtualDisk")
	procAttachVirtualDisk                = modvirtdisk.NewProc("AttachVirtualDisk")
	procGetVirtualDiskInformation        = modvirtdisk.NewProc("GetVirtualDiskInformation")
)

func CreateFileA(name *byte, access uint32, mode uint32, sa *windows.SecurityAttributes, createmode uint32, attrs uint32, templatefile windows.Handle) (handle windows.Handle, err error) {
	r0, _, e1 := syscall.Syscall9(procCreateFileA.Addr(), 7, uintptr(unsafe.Pointer(name)), uintptr(access), uintptr(mode), uintptr(unsafe.Pointer(sa)), uintptr(createmode), uintptr(attrs), uintptr(templatefile), 0, 0)
	handle = windows.Handle(r0)
	if handle == windows.InvalidHandle {
		if e1 != 0 {
			err = errnoErr(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func findFirstVolumeA(volumeName *byte, bufferLength uint32) (findVolume windows.Handle, err error) {
	r0, _, e1 := syscall.Syscall(procFindFirstVolumeA.Addr(), 2, uintptr(unsafe.Pointer(volumeName)), uintptr(bufferLength), 0)
	findVolume = windows.Handle(r0)
	if findVolume == windows.InvalidHandle {
		if e1 != 0 {
			err = errnoErr(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func findNextVolumeA(findVolume windows.Handle, volumeName *byte, bufferLength uint32) (err error) {
	r1, _, e1 := syscall.Syscall(procFindNextVolumeA.Addr(), 3, uintptr(findVolume), uintptr(unsafe.Pointer(volumeName)), uintptr(bufferLength))
	if r1 == 0 {
		if e1 != 0 {
			err = errnoErr(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func findVolumeClose(findVolume windows.Handle) (err error) {
	r1, _, e1 := syscall.Syscall(procFindVolumeClose.Addr(), 1, uintptr(findVolume), 0, 0)
	if r1 == 0 {
		if e1 != 0 {
			err = errnoErr(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func findFirstVolumeMountPointA(rootPathName *byte, volumeMountPoint *byte, bufferLength uint32) (findVolumeMountPoint windows.Handle, err error) {
	r0, _, e1 := syscall.Syscall(procFindFirstVolumeMountPointA.Addr(), 3, uintptr(unsafe.Pointer(rootPathName)), uintptr(unsafe.Pointer(volumeMountPoint)), uintptr(bufferLength))
	findVolumeMountPoint = windows.Handle(r0)
	if findVolumeMountPoint == windows.InvalidHandle {
		if e1 != 0 {
			err = errnoErr(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func findNextVolumeMountPointA(findVolumeMountPoint windows.Handle, volumeMountPoint *byte, bufferLength uint32) (err error) {
	r1, _, e1 := syscall.Syscall(procFindNextVolumeMountPointA.Addr(), 3, uintptr(findVolumeMountPoint), uintptr(unsafe.Pointer(volumeMountPoint)), uintptr(bufferLength))
	if r1 == 0 {
		if e1 != 0 {
			err = errnoErr(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func findVolumeMountPointClose(findVolumeMountPoint windows.Handle) (err error) {
	r1, _, e1 := syscall.Syscall(procFindVolumeMountPointClose.Addr(), 1, uintptr(findVolumeMountPoint), 0, 0)
	if r1 == 0 {
		if e1 != 0 {
			err = errnoErr(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func getVolumePathNamesForVolumeNameA(volumeName *byte, volumePathNames *byte, bufferLength uint32, returnLength *uint32) (err error) {
	r1, _, e1 := syscall.Syscall6(procGetVolumePathNamesForVolumeNameA.Addr(), 4, uintptr(unsafe.Pointer(volumeName)), uintptr(unsafe.Pointer(volumePathNames)), uintptr(bufferLength), uintptr(unsafe.Pointer(returnLength)), 0, 0)
	if r1 == 0 {
		if e1 != 0 {
			err = errnoErr(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func openVirtualDisk(vst *VirtualStorageType, path string, virtualDiskAccessMask uint32, flags uint32, parameters *OpenVirtualDiskParameters, handle *windows.Handle) (err error) {
	var _p0 *uint16
	_p0, err = syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	return _openVirtualDisk(vst, _p0, virtualDiskAccessMask, flags, parameters, handle)
}

func _openVirtualDisk(vst *VirtualStorageType, path *uint16, virtualDiskAccessMask uint32, flags uint32, parameters *OpenVirtualDiskParameters, handle *windows.Handle) (err error) {
	r1, _, e1 := syscall.Syscall6(procOpenVirtualDisk.Addr(), 6, uintptr(unsafe.Pointer(vst)), uintptr(unsafe.Pointer(path)), uintptr(virtualDiskAccessMask), uintptr(flags), uintptr(unsafe.Pointer(parameters)), uintptr(unsafe.Pointer(handle)))
	if r1 != 0 {
		if e1 != 0 {
			err = errnoErr(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func attachVirtualDisk(vhdh windows.Handle, sd uintptr, flags uint32, providerFlags uint32, params uintptr, overlapped uintptr) (err error) {
	r1, _, e1 := syscall.Syscall6(procAttachVirtualDisk.Addr(), 6, uintptr(vhdh), uintptr(sd), uintptr(flags), uintptr(providerFlags), uintptr(params), uintptr(overlapped))
	if r1 != 0 {
		if e1 != 0 {
			err = errnoErr(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func GetVirtualDiskInformation(vhdh windows.Handle, size *uint32, info *byte, used *uint32) (err error) {
	r1, _, e1 := syscall.Syscall6(procGetVirtualDiskInformation.Addr(), 4, uintptr(vhdh), uintptr(unsafe.Pointer(size)), uintptr(unsafe.Pointer(info)), uintptr(unsafe.Pointer(used)), 0, 0)
	if r1 != 0 {
		if e1 != 0 {
			err = errnoErr(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}
