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
	modvirtdisk = windows.NewLazySystemDLL("virtdisk.dll")

	procOpenVirtualDisk            = modvirtdisk.NewProc("OpenVirtualDisk")
	procAttachVirtualDisk          = modvirtdisk.NewProc("AttachVirtualDisk")
	procGetVirtualDiskInformation  = modvirtdisk.NewProc("GetVirtualDiskInformation")
	procGetVirtualDiskPhysicalPath = modvirtdisk.NewProc("GetVirtualDiskPhysicalPath")
)

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

func getVirtualDiskInformation(vhdh windows.Handle, size *uint32, info *byte, used *uint32) (err error) {
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

func getVirtualDiskPhysicalPath(handle windows.Handle, diskPathSizeInBytes *uint32, buffer *uint16) (err error) {
	r1, _, e1 := syscall.Syscall(procGetVirtualDiskPhysicalPath.Addr(), 3, uintptr(handle), uintptr(unsafe.Pointer(diskPathSizeInBytes)), uintptr(unsafe.Pointer(buffer)))
	if r1 != 0 {
		if e1 != 0 {
			err = errnoErr(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}
