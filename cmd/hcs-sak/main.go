// hcs-sak
//
// HCS Shim swiss army knife (sak): grab bag  of various utilities needed for
// hcsshim and containerd on Windows development
package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"syscall"

	"golang.org/x/sys/windows"

	wvhd "github.com/Microsoft/go-winio/vhd"
	"github.com/Microsoft/hcsshim/internal/vhd"
)

// to find GUID of disk:
// 1. run getvolumes twice, and find newest volume
// 2. parse GUID from embedded GPT partition in VHD:
//    https://stackoverflow.com/questions/31849488/open-attach-and-assign-a-vhd/48475371#48475371
// 3. look for diskID in findvolume enumeration:
//    https://stackoverflow.com/questions/24396644/programmatically-mount-a-microsoft-virtual-hard-drive-vhd/27611730#27611730

// https://docs.microsoft.com/en-us/windows/win32/api/winioctl/ni-winioctl-ioctl_storage_get_device_number?redirectedfrom=MSDN
// https://docs.microsoft.com/en-us/windows/win32/api/winioctl/ns-winioctl-volume_disk_extents

func main() {
	var err error

	// err = ListAllVolumes()
	// if err != nil {
	// 	fmt.Println("error: ", err)
	// 	os.Exit(1)
	// }

	if len(os.Args) != 2 {
		fmt.Printf("need to pass a path")
		os.Exit(1)
	}
	f := os.Args[1]

	err = PrintVHDInfo(f)
	if err != nil {
		fmt.Println("error: ", err)
		os.Exit(1)
	}

	err = _main(f)
	if err != nil {
		fmt.Println("error: ", err)
		os.Exit(1)

	}
}

var DiskNumberRe = regexp.MustCompile(`\\\\.\\PhysicalDrive([\d]+)`)

func _main(f string) error {
	op := wvhd.OpenVirtualDiskParameters{
		Version: 2,
		Version2: wvhd.OpenVersion2{
			ReadOnly: true,
		},
	}
	h, err := wvhd.OpenVirtualDiskWithParameters(
		f,
		wvhd.VirtualDiskAccessNone,
		wvhd.OpenVirtualDiskFlagParentCachedIO|wvhd.OpenVirtualDiskFlagIgnoreRelativeParentLocator,
		&op,
	)
	if err != nil {
		return fmt.Errorf("open virtual disk failed with: %w", err)
	}
	defer syscall.CloseHandle(h)
	wh := windows.Handle(h)

	fmt.Printf("attaching %q\n", f)
	err = wvhd.AttachVirtualDisk(h, wvhd.AttachVirtualDiskFlagReadOnly, &wvhd.AttachVirtualDiskParameters{Version: 1})
	if err != nil {
		return fmt.Errorf("attach virtual disk failed with: %w", err)
	}
	defer wvhd.DetachVirtualDisk(h)

	n, err := vhd.GetAttachedVHDDiskNumber(wh)
	if err != nil {
		return fmt.Errorf("attached virtual disk number: %w", err)
	}
	fmt.Printf("disk number %d\n", n)

	id, err := findVolume(uint32(n))
	if err != nil {
		return fmt.Errorf("could not find disk number %d: %w", n, err)
	}

	fmt.Println("vhd mount path: ", id)
	return nil

	id = id + "\\"
	// vpathptr, err := windows.UTF16PtrFromString(vpath)
	// if err != nil {
	// 	fmt.Printf("ptr 16 from string with: %v", err)
	// 	return 1
	// }

	// vpath = "\\\\?\\Volume{10bbc1b8-1583-4588-b288-0bf125cad124}\\"
	vol, err := os.OpenFile(id, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open file failed with: %w", err)
	}
	defer vol.Close()

	fileInfo, err := vol.Readdir(-1)
	if err != nil {
		return fmt.Errorf("open file failed with: %w", err)
	}

	for _, file := range fileInfo {
		fmt.Printf("- %v\n", file.Name())
	}

	// utf16DestPath := windows.StringToUTF16(f)

	// h, err := windows.CreateFile(&utf16DestPath[0], windows.GENERIC_WRITE, windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	// if err != nil {
	// 	fmt.Printf("create file failed with: %v", err)
	// 	os.Exit(1)
	// }

	// err = windows.FlushFileBuffers(h)
	// if err != nil {
	// 	fmt.Printf("file buffer flush failed with: %v", err)
	// 	os.Exit(1)
	// }
	return nil
}

func findVolume(v uint32) (id string, err error) {
	errFound := errors.New("found volume by device number")

	err = vhd.WalkVolumesA(func(vol string) error {
		n, err := vhd.GetVolumeDeviceNumber(vol)
		if err != nil {
			return err
		}
		if n == v {
			id = vol
			return errFound
		}

		return nil
	})
	if errors.Is(err, errFound) {
		return id, nil
	} else if err != nil {
		return "", fmt.Errorf("could not find volume number %d", v)
	}
	return "", err
}

func ListAllVolumes() error {
	return vhd.WalkVolumesA(func(vol string) error {
		fmt.Print(vol)

		n, err := vhd.GetVolumeDeviceNumber(vol)
		if err != nil {
			return err
		}
		fmt.Println(": ", n)

		ps, err := vhd.GetVolumePathNamesForVolumeName(vol)
		if err != nil {
			return err
		}
		for _, p := range ps {
			fmt.Println("-", p)
		}

		return nil
	})
}

func PrintVHDInfo(path string) error {
	fmt.Printf("opening vhd %q\n", path)

	op := wvhd.OpenVirtualDiskParameters{
		Version: 2,
		Version2: wvhd.OpenVersion2{
			GetInfoOnly: true,
			ReadOnly:    true,
		},
	}
	h, err := wvhd.OpenVirtualDiskWithParameters(
		path,
		wvhd.VirtualDiskAccessNone,
		wvhd.OpenVirtualDiskFlagNoParents, //|wvhd.OpenVirtualDiskFlagIgnoreRelativeParentLocator,
		&op,
	)
	if err != nil {
		return fmt.Errorf("open virtual disk failed with: %w", err)
	}
	defer syscall.CloseHandle(h)

	wh := windows.Handle(h)

	i, err := vhd.GetVirtualDiskGUID(wh)
	if err != nil {
		return err
	}
	fmt.Println("handle guid: ", i)

	i, err = vhd.GetVirtualDiskDiskGUID(wh)
	if err != nil {
		return err
	}
	fmt.Println("disk guid: ", i)

	st, err := vhd.GetVirtualDiskProviderSubtype(wh)
	if err != nil {
		return err
	}
	fmt.Println("type: ", st.String())

	if st == vhd.VirtualDiskProviderSubtypeDifferencing {
		r, ss, err := vhd.GetVirtualDiskParentLocation(wh)
		if err != nil {
			return err

		}

		fmt.Println("parent resolved: ", r)
		for _, s := range ss {
			fmt.Printf("parent path:      %s\n", s)
		}
	}

	sz, err := vhd.GetVirtualDiskSize(wh)
	if err != nil {
		return err
	}
	fmt.Printf("size: %+v\n", sz)

	return nil
}
