// Package vhd provides bindings to win32's virtual disk (vhd) and volume
// management function
package vhd

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

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
//sys getVirtualDiskInformation(vhdh windows.Handle, size *uint32, info *byte, used *uint32) (err error) [failretval != 0] = virtdisk.GetVirtualDiskInformation

type (
	GUID                      = guid.GUID
	VirtualStorageType        = winiovhd.VirtualStorageType
	OpenVirtualDiskParameters = winiovhd.OpenVirtualDiskParameters
)

type VirtualDiskInformationVersion uint32

const (
	VirtualDiskInfoVersionUnspecified VirtualDiskInformationVersion = iota
	VirtualDiskInfoVersionSize
	VirtualDiskInfoVersionIdentifier
	VirtualDiskInfoVersionParentLocation
	VirtualDiskInfoVersionParentIdentifier
	VirtualDiskInfoVersionParentTimestamp
	VirtualDiskInfoVersionVirtualStorageType
	VirtualDiskInfoVersionProviderSubtype
	VirtualDiskInfoVersionIs4kAligned
	VirtualDiskInfoVersionPhysicalDisk
	VirtualDiskInfoVersionVhdPhysicalSectorSize
	VirtualDiskInfoVersionSmallestSafeVirtualSize
	VirtualDiskInfoVersionFragmentation
	VirtualDiskInfoVersionIsLoaded
	VirtualDiskInfoVersionVirtualDiskID
	VirtualDiskInfoVersionChangeTrackingState
)

func (iv VirtualDiskInformationVersion) String() string {
	s := "get virtual disk "
	switch iv {
	case VirtualDiskInfoVersionSize:
		return s + "size"
	case VirtualDiskInfoVersionIdentifier:
		return s + "identifier"
	case VirtualDiskInfoVersionParentLocation:
		return s + "parent location"
	case VirtualDiskInfoVersionParentIdentifier:
		return s + "parent identifier"
	case VirtualDiskInfoVersionParentTimestamp:
		return s + "parent timestamp"
	case VirtualDiskInfoVersionVirtualStorageType:
		return s + "virtual storage type"
	case VirtualDiskInfoVersionProviderSubtype:
		return s + "provider subtype"
	case VirtualDiskInfoVersionIs4kAligned:
		return s + "is 4k aligned"
	case VirtualDiskInfoVersionPhysicalDisk:
		return s + "physical disk information"
	case VirtualDiskInfoVersionVhdPhysicalSectorSize:
		return s + "VHD physical selector size"
	case VirtualDiskInfoVersionSmallestSafeVirtualSize:
		return s + "safest safe virtual size"
	case VirtualDiskInfoVersionFragmentation:
		return s + "framentation percentage"
	case VirtualDiskInfoVersionIsLoaded:
		return s + "is loaded"
	case VirtualDiskInfoVersionVirtualDiskID:
		return s + "virtual disk identifier"
	case VirtualDiskInfoVersionChangeTrackingState:
		return s + "resilient change tracking state"
	default:
		return "invalid virtual disk information version "
	}
}

// https://docs.microsoft.com/en-us/windows/win32/api/virtdisk/ns-virtdisk-get_virtual_disk_info
type VirtualDiskProviderSubtype uint32

const (
	// not in the API, but add it as a return value
	VirtualDiskProviderSubtypeInvalid      VirtualDiskProviderSubtype = 0x0
	VirtualDiskProviderSubtypeFixed        VirtualDiskProviderSubtype = 0x2
	VirtualDiskProviderSubtypeDynamic      VirtualDiskProviderSubtype = 0x3
	VirtualDiskProviderSubtypeDifferencing VirtualDiskProviderSubtype = 0x4
)

func (vpst VirtualDiskProviderSubtype) String() string {
	switch vpst {
	case VirtualDiskProviderSubtypeFixed:
		return "fixed"
	case VirtualDiskProviderSubtypeDynamic:
		return "dynamically expandable (sparse) "
	case VirtualDiskProviderSubtypeDifferencing:
		return "differencing"
	default:
		return "invalid subtype"
	}
}

// todo: make a 32 bit version of this
// https://docs.microsoft.com/en-us/windows/win32/api/virtdisk/ns-virtdisk-get_virtual_disk_info
type (
	virtualDiskInformationHeader struct {
		Version VirtualDiskInformationVersion
		// alignment: union is set to 8-byte boundary (on a 64 bit system)
		_ uint32
	}

	VirtualDiskInformationGUID struct {
		virtualDiskInformationHeader

		ID GUID
	}

	VirtualDiskInformationSize struct {
		virtualDiskInformationHeader

		VirtualSize  uint64
		PhysicalSize uint64
		BlockSize    uint32
		SectorSize   uint32
	}

	VirtualDiskInformationProviderSubtype struct {
		virtualDiskInformationHeader

		ProviderSubtype VirtualDiskProviderSubtype
	}

	VirtualDiskInformationIsLoaded struct {
		virtualDiskInformationHeader

		IsLoaded bool
	}
)

type _largestVirtualDiskInformationStruct = VirtualDiskInformationSize

var _virtualDistkInformationStructBufferSize = to8byteAlignment(uint(unsafe.Sizeof(_largestVirtualDiskInformationStruct{})))

func GetVirtualDiskGUID(h windows.Handle) (GUID, error) {
	b, err := getVirtualDiskInformationFromVersion(h, VirtualDiskInfoVersionIdentifier)
	if err != nil {
		return guid.GUID{}, err
	}

	info := (*VirtualDiskInformationGUID)(unsafe.Pointer(&b[0]))
	return info.ID, nil
}

func GetVirtualDiskDiskGUID(h windows.Handle) (GUID, error) {
	b, err := getVirtualDiskInformationFromVersion(h, VirtualDiskInfoVersionVirtualDiskID)
	if err != nil {
		return guid.GUID{}, err
	}

	info := (*VirtualDiskInformationGUID)(unsafe.Pointer(&b[0]))
	return info.ID, nil
}

func GetVirtualDiskProviderSubtype(h windows.Handle) (VirtualDiskProviderSubtype, error) {
	b, err := getVirtualDiskInformationFromVersion(h, VirtualDiskInfoVersionProviderSubtype)
	if err != nil {
		return VirtualDiskProviderSubtypeInvalid, err
	}

	info := (*VirtualDiskInformationProviderSubtype)(unsafe.Pointer(&b[0]))
	return info.ProviderSubtype, nil
}

func GetVirtualDiskIsLoaded(h windows.Handle) (bool, error) {
	v := VirtualDiskInfoVersionIsLoaded
	b, err := getVirtualDiskInformationFromVersion(h, v)

	if err != nil {
		return false, err
	}

	info := (*VirtualDiskInformationIsLoaded)(unsafe.Pointer(&b[0]))
	return info.IsLoaded, nil
}

func getVirtualDiskInformationFromVersion(h windows.Handle, v VirtualDiskInformationVersion) ([]byte, error) {
	var (
		size = uint32(unsafe.Sizeof(_largestVirtualDiskInformationStruct{}))
		buff = make([]byte, size)
		head = (*virtualDiskInformationHeader)(unsafe.Pointer(&buff[0]))
	)

	head.Version = v

	fmt.Printf("%s\n", v)
	// fmt.Printf("%+v\n", buff)

	err := getVirtualDiskInformation(h, &size, &buff[0], nil)
	if err != nil {
		return buff, fmt.Errorf("%s: %w", v.String(), err)
	}

	// fmt.Printf("%+v\n", buff)

	return buff, nil
}
