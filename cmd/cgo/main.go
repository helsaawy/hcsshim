//go:build windows

package main

// #include <windows.h>
import "C"

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Println("got error:", err)
		os.Exit(1)
	}
}

func run() error {
	i := C.struct__OSVERSIONINFOW{}
	i.dwOSVersionInfoSize = C.DWORD(C.sizeof_struct__OSVERSIONINFOW)
	_, err := C.GetVersionExW(&i)
	fmt.Printf("version: %d.%d.%d\n", i.dwMajorVersion, i.dwMinorVersion, i.dwBuildNumber)
	return err
}
