//go:build windows && mage

package main

import (
	"os"

	"github.com/magefile/mage/sh"
)

//nolint:unused,varcheck
const (
	shellPipelineAnd = "&&"

	archiveExt = ".zip"
	binaryExt  = ".exe" // replacement for GOEXE
)

//nolint:unused,varcheck
var (
	shellCmd   = os.Getenv("COMSPEC")
	shellFlags = []string{"/e:on", "/c"}
)

// starting from 17063, windows has tar.exe and curl.exe built into CMD.
// https://learn.microsoft.com/en-us/virtualization/community/team-blog/2017/20171219-tar-and-curl-come-to-windows

// mergeTarFiles combines multiple tar(.gz) files together into one.
func mergeTarFiles(dst string, srcs ...string) error {
	//intermediary unpacking of base and delta should be more space-efficient and prevent duplicates.
	// however, we would loose all the permissions.
	//
	// could use mtree to enumerate permissions and recreate tar after unpacking, but since
	// duplicate entries in an mtree file will get added repeatedly, would still need to manually
	// de-duplicate (or construct an in-memory representation of the data)
	//
	// could also try to create an ext4 formatted file-system or use a uVM to perform the merge...

	// This does not flatten the tar file, so there will be duplicate directories (ie, /bin/)
	for i, src := range srcs {
		srcs[i] = "@" + src
	}
	// Since the base and delta tars do not have the uname set, this spews a lot of errors
	// of the form:
	//
	// 		tar.exe: <file>: Can't translate uname '(null)' to UTF-8
	//
	// So ignore the stdout/err channels
	_, err := sh.Exec(nil, nil, nil, "tar.exe",
		append([]string{"-cf", dst, "--gid=0", "--uid=0"}, srcs...)...)
	return err
}

func convertTarToInitramfs(dst, src string) error {
	return sh.Run("tar.exe", "-czf", dst, "--format=newc", "@"+src)
}
