// Package vhd provides bindings to win32's virtual disk (vhd) and volume
// management function
package vhd

import (
	"errors"
	"fmt"
	"syscall"
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

// type aliases from imported packages
type (
	GUID                      = guid.GUID
	VirtualStorageType        = winiovhd.VirtualStorageType
	OpenVirtualDiskParameters = winiovhd.OpenVirtualDiskParameters
)

const ErrInvalidArgument syscall.Errno = 0x20000027

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
	}

	VirtualDiskInformationGUID struct {
		ID GUID
	}

	VirtualDiskInformationSize struct {
		VirtualSize  uint64
		PhysicalSize uint64
		BlockSize    uint32
		SectorSize   uint32
	}

	VirtualDiskInformationParentLocation struct {
		ParentResolved bool
		// bools in win32 are ints, ie 4 bytes
		// https://docs.microsoft.com/en-us/windows/win32/winprog/windows-data-types#bool
		// todo: is a bool guaranteed to be less than 4 bytes by go
		_ [4 - unsafe.Sizeof(true)]byte
		// this will be the start of a variable length array
		ParentLocationBuffer uint16
	}

	// todo: are these wrappers needed?

	VirtualDiskInformationProviderSubtype struct {
		ProviderSubtype VirtualDiskProviderSubtype
	}

	VirtualDiskInformationIsLoaded struct {
		IsLoaded bool
	}
)

type largestVirtualDiskInformationStruct = VirtualDiskInformationSize

const (
	_largestVirtualDiskInformationStructAlignment = unsafe.Alignof(largestVirtualDiskInformationStruct{})
	_largestVirtualDiskInformationStructSize      = unsafe.Sizeof(largestVirtualDiskInformationStruct{})
	// GetVirtualDiskInfo union will be aligned to 4- or 8-byte boundary (on 32- or 64-bit system).
	// Adding a `_ [0]byte` field to the virtualDiskInformationHeader struct will still causes
	// the size to increase from 4 to 8, which induces improper padding.
	// Therefore, account for the padding at buffer allocation.
	_virtualDiskInformationHeaderPadding = _largestVirtualDiskInformationStructAlignment - unsafe.Sizeof(VirtualDiskInfoVersionUnspecified)
	_virtualDiskInformationHeaderSize    = unsafe.Sizeof(virtualDiskInformationHeader{}) + _virtualDiskInformationHeaderPadding
)

func GetVirtualDiskSize(h windows.Handle) (VirtualDiskInformationSize, error) {
	b, err := getVirtualDiskInformationFromVersion(h, VirtualDiskInfoVersionSize)
	if err != nil {
		return VirtualDiskInformationSize{}, err
	}

	sz := (*VirtualDiskInformationSize)(unsafe.Pointer(&b[0]))
	return *sz, nil
}

func GetVirtualDiskGUID(h windows.Handle) (GUID, error) {
	b, err := getVirtualDiskInformationFromVersion(h, VirtualDiskInfoVersionIdentifier)
	if err != nil {
		return guid.GUID{}, err
	}

	id := (*VirtualDiskInformationGUID)(unsafe.Pointer(&b[0]))
	return id.ID, nil
}

func GetVirtualDiskParentLocation(h windows.Handle) (bool, []string, error) {
	b, err := getVirtualDiskInformationFromVersion(h, VirtualDiskInfoVersionParentLocation)
	if err != nil {
		return false, []string{}, err
	}

	info := (*VirtualDiskInformationParentLocation)(unsafe.Pointer(&b[0]))

	// todo: !ParentResolved, parse full array of returned paths
	sb := unsafe.Slice(&info.ParentLocationBuffer, uintptr(len(b))-unsafe.Offsetof(info.ParentLocationBuffer))
	s := syscall.UTF16ToString(sb)
	return info.ParentResolved, []string{s}, nil
}

// a unique ID that is constnat for the VHD
func GetVirtualDiskDiskGUID(h windows.Handle) (GUID, error) {
	b, err := getVirtualDiskInformationFromVersion(h, VirtualDiskInfoVersionVirtualDiskID)
	if err != nil {
		return guid.GUID{}, err
	}

	id := (*VirtualDiskInformationGUID)(unsafe.Pointer(&b[0]))
	return id.ID, nil
}

func GetVirtualDiskProviderSubtype(h windows.Handle) (VirtualDiskProviderSubtype, error) {
	b, err := getVirtualDiskInformationFromVersion(h, VirtualDiskInfoVersionProviderSubtype)
	if err != nil {
		return VirtualDiskProviderSubtypeInvalid, err
	}

	st := (*VirtualDiskProviderSubtype)(unsafe.Pointer(&b[0]))
	return *st, nil
}

func GetVirtualDiskIsLoaded(h windows.Handle) (bool, error) {
	v := VirtualDiskInfoVersionIsLoaded
	b, err := getVirtualDiskInformationFromVersion(h, v)

	if err != nil {
		return false, err
	}

	loaded := (*bool)(unsafe.Pointer(&b[0]))
	return *loaded, nil
}

// getVirtualDiskInformationFromVersion ...
// payloadSize is the size of data after the header. The size used will be
//   Sizeof(header) + max(payloadSize, Sizeof(minimumRequiredPayloadSize))
func getVirtualDiskInformationFromVersion(h windows.Handle, v VirtualDiskInformationVersion) (buff []byte, err error) {
	// its annoying to type ...
	const hsz = _virtualDiskInformationHeaderSize
	var (
		size = uint32(hsz + _largestVirtualDiskInformationStructAlignment)
		// todo: is `used` valuable?
		used uint32
	)

	// fmt.Printf("%s\n", v)
	// fmt.Printf("%+v\n", buff)

	// max 5 re-tries for invalid arguments
	for i := 0; i < 5; i++ {
		buff = make([]byte, size)
		hdr := (*virtualDiskInformationHeader)(unsafe.Pointer(&buff[0]))
		hdr.Version = v

		err := getVirtualDiskInformation(h, &size, &buff[0], &used)
		if errors.Is(err, ErrInvalidArgument) && int(size) != len(buff) {
			continue
		}
		break
	}

	if err != nil {
		err = fmt.Errorf("%s: %w", v.String(), err)
	}

	return buff[hsz:], err
}
