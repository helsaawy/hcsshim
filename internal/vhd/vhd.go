// Package vhd provides bindings to win32's virtual disk (vhd) and volume
// management function
package vhd

import (
	"github.com/Microsoft/go-winio/pkg/guid"
	winiovhd "github.com/Microsoft/go-winio/vhd"
)

//go:generate go run ../../mksyscall_windows.go -output zsyscall_windows.go vhd.go

//
// win32 apis
//

//sys CreateFileA(name *byte, access uint32, mode uint32, sa *windows.SecurityAttributes, createmode uint32, attrs uint32, templatefile windows.Handle) (handle windows.Handle, err error) [failretval==windows.InvalidHandle] = CreateFileA

//
// volume management
//

//sys findFirstVolumeA(volumeName *byte, bufferLength uint32) (findVolume windows.Handle, err error) [failretval==windows.InvalidHandle] = FindFirstVolumeA
//sys findNextVolumeA(findVolume windows.Handle, volumeName *byte, bufferLength uint32) (err error) = FindNextVolumeA
//sys findVolumeClose(findVolume windows.Handle) (err error) = FindVolumeClose
//sys findFirstVolumeMountPointA(rootPathName *byte, volumeMountPoint *byte, bufferLength uint32) (findVolumeMountPoint windows.Handle, err error) [failretval==windows.InvalidHandle] = FindFirstVolumeMountPointA

//sys findNextVolumeMountPointA(findVolumeMountPoint windows.Handle, volumeMountPoint *byte, bufferLength uint32) (err error) = FindNextVolumeMountPointA
//sys findVolumeMountPointClose(findVolumeMountPoint windows.Handle) (err error) = FindVolumeMountPointClose
//sys getVolumePathNamesForVolumeNameA(volumeName *byte, volumePathNames *byte, bufferLength uint32, returnLength *uint32) (err error) = GetVolumePathNamesForVolumeNameA

//
// virtual disk (vhds)
//

//sys openVirtualDisk(vst *VirtualStorageType, path string, virtualDiskAccessMask uint32, flags uint32, parameters *OpenVirtualDiskParameters, handle *windows.Handle) (err error) [failretval != 0] = virtdisk.OpenVirtualDisk
//sys attachVirtualDisk(vhdh windows.Handle, sd uintptr, flags uint32, providerFlags uint32, params uintptr, overlapped uintptr) (err error) [failretval != 0] = virtdisk.AttachVirtualDisk

//sys GetVirtualDiskInformation(vhdh windows.Handle, size *uint32, info *byte , used *uint32) (err error) [failretval != 0] = virtdisk.GetVirtualDiskInformation
// getVirtualDiskInformation(vhdh windows.Handle, size *uint32, info *VirtualDiskInformationGUID , used *uint32) (err error) [failretval != 0] = virtdisk.GetVirtualDiskInformation

type (
	GUID                      = guid.GUID
	VirtualStorageType        = winiovhd.VirtualStorageType
	OpenVirtualDiskParameters = winiovhd.OpenVirtualDiskParameters
)

// type (
// VirtualDiskInformationVersion uint32

// VirtualDiskInformationGUID struct {
// 	Version VirtualDiskInformationVersion
// 	ID      GUID
// }

// for when generics come :'(
// VirtualDiskInformationGUID GUID
// VirtualDiskInformationIs4kAligned bool
// VirtualDiskInformationIsLoaded bool

// VirtualDiskInformationUnion interface {
// 	VirtualDiskInformationGUID |
// 	VirtualDiskInformationIs4kAligned |
// 	VirtualDiskInformationIsLoaded
// }

// VirtualDiskInformation[E VirtualDiskInformationUnion] struct {
// 	Version VirtualDiskInformationVersion
// 	E
// }
// )

// const (
// VirtualDiskInfoVersionUnspecified VirtualDiskInformationVersion = iota
// VirtualDiskInfoVersionSize
// VirtualDiskInfoVersionIdentifier
// VirtualDiskInfoVersionParentLocation
// VirtualDiskInfoVersionParentIdentifier
// VirtualDiskInfoVersionParentTimestamp
// VirtualDiskInfoVersionVirtualStorageType
// VirtualDiskInfoVersionProviderSubtype
// VirtualDiskInfoVersionIs4kAligned
// VirtualDiskInfoVersionPhysicalDisk
// VirtualDiskInfoVersionVhdPhysicalSectorSize
// VirtualDiskInfoVersionSmallestSafeVirtualSize
// VirtualDiskInfoVersionFragmentation
// VirtualDiskInfoVersionIsLoaded
// VirtualDiskInfoVersionVirtualDiskID
// VirtualDiskInfoVersionChangeTrackingState
// )

// func GetVirtualDiskGUID(h windows.Handle) (GUID, error) {
// 	var (
// 		info = VirtualDiskInformationGUID{
// 			Version: VirtualDiskInfoVersionIdentifier,
// 		}
// 		s = uint32(unsafe.Sizeof(info))
// 		u uint32
// 	)

// 	fmt.Printf(">> size is %v", s)

// 	err := getVirtualDiskInformation(h, &s, &info, &u)
// 	if err != nil {
// 		return guid.GUID{}, fmt.Errorf("getVirtualDiskInformation: %w", err)
// 	}

// 	fmt.Printf(">> size was %v and used was %v", s, u)

// 	return info.ID, nil
// }
