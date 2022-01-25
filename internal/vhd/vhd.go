// Package vhd provides bindings to win32's virtual disk (vhd) and volume
// management function
package vhd

import (
	"fmt"
	"unsafe"

	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/Microsoft/go-winio/vhd"
	"golang.org/x/sys/windows"
)

//go:generate go run ../../mksyscall_windows.go -output zsyscall_windows.go vhd.go

//sys FindFirstVolume(volumeName *uint16, bufferLength uint32) (handle windows.Handle, err error) [failretval==windows.InvalidHandle] = FindFirstVolumeA
//sys FindFirstVolumeMountPoint(rootPathName *uint16, volumeMountPoint *uint16, bufferLength uint32) (handle windows.Handle, err error) [failretval==windows.InvalidHandle] = FindFirstVolumeMountPointA
//sys FindNextVolume(findVolume windows.Handle, volumeName *uint16, bufferLength uint32) (err error) = FindNextVolumeA
//sys FindNextVolumeMountPoint(findVolumeMountPoint windows.Handle, volumeMountPoint *uint16, bufferLength uint32) (err error) = FindNextVolumeMountPointA
//sys FindVolumeClose(findVolume windows.Handle) (err error)
//sys FindVolumeMountPointClose(findVolumeMountPoint windows.Handle) (err error)

//sys openVirtualDisk(vst *VirtualStorageType, path string, virtualDiskAccessMask uint32, flags uint32, parameters *OpenVirtualDiskParameters, handle *windows.Handle) (err error) [failretval != 0] = virtdisk.OpenVirtualDisk
//sys attachVirtualDisk(vhdh windows.Handle, sd uintptr, flags uint32, providerFlags uint32, params uintptr, overlapped uintptr) (err error) [failretval != 0] = virtdisk.AttachVirtualDisk
//sys getVirtualDiskInformation(vhdh windows.Handle, size *uint32, info uintptr, used *uint32) (err error) [failretval != 0] = virtdisk.GetVirtualDiskInformation

type (
	GUID                      = guid.GUID
	VirtualStorageType        = vhd.VirtualStorageType
	OpenVirtualDiskParameters = vhd.OpenVirtualDiskParameters
)

type (
	GetVirtualDiskInformationVersion uint32

	GetVirtualDiskInformationGUID struct {
		Version GetVirtualDiskInformationVersion
		ID      GUID
	}
)

const (
	GetVirtualDiskInfounspecified GetVirtualDiskInformationVersion = iota
	GetVirtualDiskInfoSize
	GetVirtualDiskInfoIdentifier
	GetVirtualDiskInfoParentLocation
	GetVirtualDiskInfoParentIdentifier
	GetVirtualDiskInfoParentTimestamp
	GetVirtualDiskInfoVirtualStorageType
	GetVirtualDiskInfoProviderSubtype
	GetVirtualDiskInfoIs4kAligned
	GetVirtualDiskInfoPhysicalDisk
	GetVirtualDiskInfoVhdPhysicalSectorSize
	GetVirtualDiskInfoSmallestSafeVirtualSize
	GetVirtualDiskInfoFragmentation
	GetVirtualDiskInfoIsLoaded
	GetVirtualDiskInfoVirtualDiskID
	GetVirtualDiskInfoChangeTrackingState
)

func GetVirtualDiskGUID(h windows.Handle) (GUID, error) {
	var (
		info = GetVirtualDiskInformationGUID{
			Version: GetVirtualDiskInfoIdentifier,
		}
		s = uint32(unsafe.Sizeof(info))
		u uint32
	)

	err := getVirtualDiskInformation(h, &s, uintptr(unsafe.Pointer(&info)), &u)
	if err != nil {
		return guid.GUID{}, fmt.Errorf("getVirtualDiskInformation: %w", err)
	}

	fmt.Printf(">> size was %v and used was %v", s, u)

	return info.ID, nil
}
