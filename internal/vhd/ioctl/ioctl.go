package ioctl

// https://docs.microsoft.com/en-us/windows/win32/api/ioapiset/nf-ioapiset-deviceiocontrol

const (
	_FILE_ANY_ACCESS     uint32 = 0
	_FILE_SPECIAL_ACCESS uint32 = _FILE_ANY_ACCESS
	_FILE_READ_ACCESS    uint32 = 0x0001
	_FILE_WRITE_ACCESS   uint32 = 0x0002

	_METHOD_BUFFERED   uint32 = 0
	_METHOD_IN_DIRECT  uint32 = 1
	_METHOD_OUT_DIRECT uint32 = 2
	_METHOD_NEITHER    uint32 = 3

	_IOCTL_STORAGE_BASE uint32 = _FILE_DEVICE_MASS_STORAGE
	_IOCTL_VOLUME_BASE  uint32 = 'V'
)

const (
	_FILE_DEVICE_BEEP uint32 = iota + 1
	_FILE_DEVICE_CD_ROM
	_FILE_DEVICE_CD_ROM_FILE_SYSTEM
	_FILE_DEVICE_CONTROLLER
	_FILE_DEVICE_DATALINK
	_FILE_DEVICE_DFS
	_FILE_DEVICE_DISK
	_FILE_DEVICE_DISK_FILE_SYSTEM
	_FILE_DEVICE_FILE_SYSTEM
	_FILE_DEVICE_INPORT_PORT
	_FILE_DEVICE_KEYBOARD
	_FILE_DEVICE_MAILSLOT
	_FILE_DEVICE_MIDI_IN
	_FILE_DEVICE_MIDI_OUT
	_FILE_DEVICE_MOUSE
	_FILE_DEVICE_MULTI_UNC_PROVIDER
	_FILE_DEVICE_NAMED_PIPE
	_FILE_DEVICE_NETWORK
	_FILE_DEVICE_NETWORK_BROWSER
	_FILE_DEVICE_NETWORK_FILE_SYSTEM
	_FILE_DEVICE_NULL
	_FILE_DEVICE_PARALLEL_PORT
	_FILE_DEVICE_PHYSICAL_NETCARD
	_FILE_DEVICE_PRINTER
	_FILE_DEVICE_SCANNER
	_FILE_DEVICE_SERIAL_MOUSE_PORT
	_FILE_DEVICE_SERIAL_PORT
	_FILE_DEVICE_SCREEN
	_FILE_DEVICE_SOUND
	_FILE_DEVICE_STREAMS
	_FILE_DEVICE_TAPE
	_FILE_DEVICE_TAPE_FILE_SYSTEM
	_FILE_DEVICE_TRANSPORT
	_FILE_DEVICE_UNKNOWN
	_FILE_DEVICE_VIDEO
	_FILE_DEVICE_VIRTUAL_DISK
	_FILE_DEVICE_WAVE_IN
	_FILE_DEVICE_WAVE_OUT
	_FILE_DEVICE_8042_PORT
	_FILE_DEVICE_NETWORK_REDIRECTOR
	_FILE_DEVICE_BATTERY
	_FILE_DEVICE_BUS_EXTENDER
	_FILE_DEVICE_MODEM
	_FILE_DEVICE_VDM
	_FILE_DEVICE_MASS_STORAGE
	_FILE_DEVICE_SMB
	_FILE_DEVICE_KS
	_FILE_DEVICE_CHANGER
	_FILE_DEVICE_SMARTCARD
	_FILE_DEVICE_ACPI
	_FILE_DEVICE_DVD
	_FILE_DEVICE_FULLSCREEN_VIDEO
	_FILE_DEVICE_DFS_FILE_SYSTEM
	_FILE_DEVICE_DFS_VOLUME
	_FILE_DEVICE_SERENUM
	_FILE_DEVICE_TERMSRV
	_FILE_DEVICE_KSEC
	_FILE_DEVICE_FIPS
	_FILE_DEVICE_INFINIBAND
)

type IOControlCode uint32

var (
	IOCTL_STORAGE_CHECK_VERIFY            = ctlcode(_IOCTL_STORAGE_BASE, 0x0200, _METHOD_BUFFERED, _FILE_READ_ACCESS)
	IOCTL_STORAGE_CHECK_VERIFY2           = ctlcode(_IOCTL_STORAGE_BASE, 0x0200, _METHOD_BUFFERED, _FILE_ANY_ACCESS)
	IOCTL_STORAGE_MEDIA_REMOVAL           = ctlcode(_IOCTL_STORAGE_BASE, 0x0201, _METHOD_BUFFERED, _FILE_READ_ACCESS)
	IOCTL_STORAGE_EJECT_MEDIA             = ctlcode(_IOCTL_STORAGE_BASE, 0x0202, _METHOD_BUFFERED, _FILE_READ_ACCESS)
	IOCTL_STORAGE_LOAD_MEDIA              = ctlcode(_IOCTL_STORAGE_BASE, 0x0203, _METHOD_BUFFERED, _FILE_READ_ACCESS)
	IOCTL_STORAGE_LOAD_MEDIA2             = ctlcode(_IOCTL_STORAGE_BASE, 0x0203, _METHOD_BUFFERED, _FILE_ANY_ACCESS)
	IOCTL_STORAGE_RESERVE                 = ctlcode(_IOCTL_STORAGE_BASE, 0x0204, _METHOD_BUFFERED, _FILE_READ_ACCESS)
	IOCTL_STORAGE_RELEASE                 = ctlcode(_IOCTL_STORAGE_BASE, 0x0205, _METHOD_BUFFERED, _FILE_READ_ACCESS)
	IOCTL_STORAGE_FIND_NEW_DEVICES        = ctlcode(_IOCTL_STORAGE_BASE, 0x0206, _METHOD_BUFFERED, _FILE_READ_ACCESS)
	IOCTL_STORAGE_EJECTION_CONTROL        = ctlcode(_IOCTL_STORAGE_BASE, 0x0250, _METHOD_BUFFERED, _FILE_ANY_ACCESS)
	IOCTL_STORAGE_MCN_CONTROL             = ctlcode(_IOCTL_STORAGE_BASE, 0x0251, _METHOD_BUFFERED, _FILE_ANY_ACCESS)
	IOCTL_STORAGE_GET_MEDIA_TYPES         = ctlcode(_IOCTL_STORAGE_BASE, 0x0300, _METHOD_BUFFERED, _FILE_ANY_ACCESS)
	IOCTL_STORAGE_GET_MEDIA_TYPES_EX      = ctlcode(_IOCTL_STORAGE_BASE, 0x0301, _METHOD_BUFFERED, _FILE_ANY_ACCESS)
	IOCTL_STORAGE_GET_MEDIA_SERIAL_NUMBER = ctlcode(_IOCTL_STORAGE_BASE, 0x0304, _METHOD_BUFFERED, _FILE_ANY_ACCESS)
	IOCTL_STORAGE_GET_HOTPLUG_INFO        = ctlcode(_IOCTL_STORAGE_BASE, 0x0305, _METHOD_BUFFERED, _FILE_ANY_ACCESS)
	IOCTL_STORAGE_SET_HOTPLUG_INFO        = ctlcode(_IOCTL_STORAGE_BASE, 0x0306, _METHOD_BUFFERED, _FILE_READ_ACCESS|_FILE_WRITE_ACCESS)
	IOCTL_STORAGE_RESET_BUS               = ctlcode(_IOCTL_STORAGE_BASE, 0x0400, _METHOD_BUFFERED, _FILE_READ_ACCESS)
	IOCTL_STORAGE_RESET_DEVICE            = ctlcode(_IOCTL_STORAGE_BASE, 0x0401, _METHOD_BUFFERED, _FILE_READ_ACCESS)
	IOCTL_STORAGE_BREAK_RESERVATION       = ctlcode(_IOCTL_STORAGE_BASE, 0x0405, _METHOD_BUFFERED, _FILE_READ_ACCESS)
	IOCTL_STORAGE_GET_DEVICE_NUMBER       = ctlcode(_IOCTL_STORAGE_BASE, 0x0420, _METHOD_BUFFERED, _FILE_ANY_ACCESS)
	IOCTL_STORAGE_PREDICT_FAILURE         = ctlcode(_IOCTL_STORAGE_BASE, 0x0440, _METHOD_BUFFERED, _FILE_ANY_ACCESS)
	IOCTL_STORAGE_READ_CAPACITY           = ctlcode(_IOCTL_STORAGE_BASE, 0x0450, _METHOD_BUFFERED, _FILE_READ_ACCESS)
	IOCTL_VOLUME_GET_VOLUME_DISK_EXTENTS  = ctlcode(_IOCTL_VOLUME_BASE, 0x0000, _METHOD_BUFFERED, _FILE_ANY_ACCESS)
	IOCTL_VOLUME_IS_CLUSTERED             = ctlcode(_IOCTL_VOLUME_BASE, 0x0012, _METHOD_BUFFERED, _FILE_ANY_ACCESS)
)

func ctlcode(deviceType, function, method, access uint32) IOControlCode {
	return IOControlCode(((deviceType) << 16) | ((access) << 14) | ((function) << 2) | (method))
}

// https://docs.microsoft.com/en-us/windows/win32/api/winioctl/ns-winioctl-disk_extent
type DiskExtent struct {
	DiskNumber                   uint32
	StartingOffset, ExtendLength uint64
}

// https://docs.microsoft.com/en-us/windows/win32/api/winioctl/ns-winioctl-volume_disk_extents
type VolumeDiskExtents struct {
	// todo, make this generic in volume disk extent array size
	Num         uint32
	DiskExtents [1]DiskExtent
}
