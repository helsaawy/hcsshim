package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/Microsoft/go-winio/vhd"
	"golang.org/x/sys/windows"
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
	// if len(os.Args) != 2 {
	// 	fmt.Printf("need to pass a path")
	// 	os.Exit(1)
	// }
	// f := os.Args[1]

	// ctx := context.Background()

	// fmt.Printf("opening vhd %q\n", f)
	// err := vhd.AttachVhd(f)
	// if err != nil {
	// 	fmt.Printf("open virtual disk failed with: %v", err)
	// 	os.Exit(1)
	// }

	// vpath, err := wclayer.GetLayerMountPath(ctx, f)
	// if err != nil {
	// 	fmt.Printf("get layer mount path failed with: %v", err)
	// 	os.Exit(1)
	// }

	// fmt.Printf("mounted vhd to %q", vpath)

	os.Exit(_main())
}

func _main() int {
	if len(os.Args) != 2 {
		fmt.Printf("need to pass a path")
		return 1
	}
	f := os.Args[1]

	// ctx := context.Background()

	fmt.Printf("opening vhd %q\n", f)

	// err := vhd.AttachVhd(f)
	// if err != nil {
	// 	fmt.Printf("open virtual disk failed with: %v", err)
	// 	return 1
	// }

	// op := vhd.OpenVirtualDiskParameters{Version: 2}
	h, err := vhd.OpenVirtualDisk(
		f,
		vhd.VirtualDiskAccessNone,
		vhd.OpenVirtualDiskFlagNone,
		// vhd.OpenVirtualDiskFlagCachedIO|vhd.OpenVirtualDiskFlagIgnoreRelativeParentLocator,
	)
	if err != nil {
		fmt.Printf("open virtual disk failed with: %v", err)
		return 1
	}
	defer syscall.CloseHandle(h)

	fmt.Printf("attaching %q\n", f)
	err = vhd.AttachVirtualDisk(h, vhd.AttachVirtualDiskFlagNone, &vhd.AttachVirtualDiskParameters{Version: 1})
	if err != nil {
		fmt.Printf("attach virtual disk failed with: %v", err)
		return 1
	}
	defer vhd.DetachVirtualDisk(h)

	vpath, err := vhd.GetVirtualDiskPhysicalPath(h)
	if err != nil {
		fmt.Printf("get layer mount path failed with: %v", err)
		return 1
	}

	fmt.Printf("mounted vhd to %q\n", vpath)

	if vpath[len(vpath)-1] != '\\' {
		vpath = vpath + "\\"
	}

	// fmt.Println(vpath + ":")
	ListAllVolumes()

	vpathptr, err := windows.UTF16PtrFromString(vpath)
	if err != nil {
		fmt.Printf("ptr 16 from string with: %v", err)
		return 1
	}

	vb := make([]uint16, 256, 256)
	nb := make([]uint16, 256, 256)
	err = windows.GetVolumeNameForVolumeMountPoint(vpathptr, &vb[0], 256)
	// err = windows.GetVolumeInformationByHandle(windows.Handle(h), &vb[0], 256, nil, nil, nil, &nb[0], 256)
	if err != nil {
		fmt.Printf("get vol info failed with: %v", err)
		return 1
	}

	fmt.Println(windows.UTF16PtrToString(&vb[0]))
	fmt.Println(windows.UTF16PtrToString(&nb[0]))

	vpath = "\\\\?\\Volume{10bbc1b8-1583-4588-b288-0bf125cad124}\\"
	vol, err := os.OpenFile(vpath, os.O_RDONLY, 0)
	if err != nil {
		fmt.Printf("open file failed with: %v", err)
		return 1
	}
	defer vol.Close()

	fileInfo, err := vol.Readdir(-1)
	if err != nil {
		fmt.Printf("open file failed with: %v", err)
		return 1
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
	return 0
}
func ListAllVolumes() int {
	const n = 256
	var buff = make([]uint16, n, n)

	h, err := windows.FindFirstVolume(&buff[0], n)
	if err != nil {
		fmt.Printf("could not find first volume: %v", err)
		// os.Exit(1)
		return 1
	}

	for {
		printUtf(buff)
		printpathnames(&buff[0])

		err = windows.FindNextVolume(h, &buff[0], n)
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			err := windows.FindVolumeClose(h)
			if err != nil {
				fmt.Printf("could not close find volume handle: %v", err)
				os.Exit(1)
			}
			break
		} else if err != nil {
			fmt.Printf("could not find next volume: %v", err)
			return 1
		}
	}
	return 0
}

func printUtf(b []uint16) {
	s := windows.UTF16ToString(b)
	fmt.Println(s)
}

func printpathnames(b *uint16) {
	const np = 1024
	var pl uint32
	var pb = make([]uint16, np, np)

	err := windows.GetVolumePathNamesForVolumeName(b, &pb[0], uint32(np), &pl)
	if err != nil {
		fmt.Printf("could not get path names: %v", err)
		os.Exit(1)
	}

	if pl < 2 {
		fmt.Println("<no mount points found>")
	}

	for i := uint32(0); i < pl; {
		s := windows.UTF16PtrToString(&pb[i])
		if s == "" {
			break
		}
		fmt.Print("- ")
		fmt.Println(s)
		for pb[i] != 0 && i < pl {
			i++
		}
	}
}
