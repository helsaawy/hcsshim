// hcs-sak
//
// HCS Shim swiss army knife (sak): grab bag  of various utilities needed for
// hcsshim and containerd on Windows development
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"syscall"

	"golang.org/x/sys/windows"

	wvhd "github.com/Microsoft/go-winio/vhd"
	"github.com/Microsoft/hcsshim/internal/vhd"
)

// todo:
//* add logging and debug statements
//** switch to something that isnt logrus
//*** default "log" package?
//** debug flag
//* add CLI apps and commands and help and usage
//* have functions (list vols, VHD info) return populated structs that are then printed
//* add depth and reparse options to list vhd dir

func main() {
	var err error
	ctx := context.Background()

	err = ListAllVolumes(ctx)
	if err != nil {
		fmt.Println("error: ", err)
		os.Exit(1)
	}

	if len(os.Args) != 2 {
		fmt.Printf("need to pass a path")
		os.Exit(1)
	}
	f := os.Args[1]

	err = PrintVHDInfo(ctx, f)
	if err != nil {
		fmt.Println("error: ", err)
		os.Exit(1)
	}

	err = PrintDirs(ctx, f)
	if err != nil {
		fmt.Println("error: ", err)
		os.Exit(1)
	}
}

var DiskNumberRe = regexp.MustCompile(`\\\\.\\PhysicalDrive([\d]+)`)

func PrintDirs(ctx context.Context, f string) error {
	op := wvhd.OpenVirtualDiskParameters{
		Version: 2,
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

	n, err := vhd.GetAttachedVHDDiskNumber(ctx, wh)
	if err != nil {
		return fmt.Errorf("attached virtual disk number: %w", err)
	}
	fmt.Printf("disk number %d\n", n)

	id, err := findVolume(ctx, n)
	if err != nil {
		return fmt.Errorf("could not find disk number %d: %w", n, err)
	}

	fmt.Println("vhd mount path: ", id)

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

	return nil
}

func findVolume(ctx context.Context, v uint32) (id string, err error) {
	errFound := errors.New("found volume by device number")

	err = vhd.WalkVolumes(ctx, func(ctx context.Context, vol string) error {
		n, err := vhd.GetVolumeDeviceNumber(ctx, vol)
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

func ListAllVolumes(ctx context.Context) error {
	return vhd.WalkVolumes(ctx, func(ctx context.Context, vol string) error {
		Display(ctx, "name", vol, -1)

		n, err := vhd.GetVolumeDeviceNumber(ctx, vol)
		if err != nil {
			return err
		}
		Display(ctx, "dev number", n, -1)

		ps, err := vhd.GetVolumePathNamesForVolumeName(ctx, vol)
		if err != nil {
			return err
		}
		switch len(ps) {
		case 0:
		case 1:
			Display(ctx, "mount path", ps[0], -1)
		default:
			Display(ctx, "mount paths", ps[0], -1)
			for _, p := range ps[1:] {
				Display(ctx, "", p, -1)
			}
		}

		fmt.Println()

		return nil
	})
}

func PrintVHDInfo(ctx context.Context, path string) error {
	Display(ctx, "vhd", path, -1)

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

	i, err := vhd.GetVirtualDiskGUID(ctx, wh)
	if err != nil {
		return err
	}
	Display(ctx, "handle guid", i, -1)

	i, err = vhd.GetVirtualDiskDiskGUID(ctx, wh)
	if err != nil {
		return err
	}
	Display(ctx, "disk guid", i, -1)

	st, err := vhd.GetVirtualDiskProviderSubtype(ctx, wh)
	if err != nil {
		return err
	}
	Display(ctx, "type", st.String(), -1)

	if st == vhd.VirtualDiskProviderSubtypeDifferencing {
		r, ss, err := vhd.GetVirtualDiskParentLocation(ctx, wh)
		if err != nil {
			return err

		}

		Display(ctx, "parent resolved", r, -1)
		for _, s := range ss {
			Display(ctx, "parent path", s, -1)
		}
	}

	sz, err := vhd.GetVirtualDiskSize(ctx, wh)
	if err != nil {
		return err
	}
	Display(ctx, "size", sz, -1)

	return nil
}
