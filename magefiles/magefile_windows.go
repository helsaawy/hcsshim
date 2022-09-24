//go:build windows && mage

package main

import (
	"os"
	"path/filepath"

	"github.com/magefile/mage/sh"
)

//nolint:unused,varcheck
var (
	shellCmd         = filepath.Join(os.Getenv("SystemRoot"), `System32\cmd.exe`)
	shellFlags       = []string{"/e:on", "/c"}
	shellPipelineAnd = "&&"
)

// mergeTarFiles combines multiple tar(.gz) files together into one.
func mergeTarFiles(dst string, srcs ...string) error {
	//intermediary unpacking of base and delta saves ~4% (and prevents duplicates)
	//todo: use mtree to enumerate permissions and recreate tar after unpacking (after de-dup)

	// This does not flatten the tar file, so there will be duplicate directories
	// Unpacking then repacking will lose all permissions, and would require storing it somewhere
	args := make([]string, 0, len(srcs)+4)
	args = append(args, "-cf", dst, "--gid=0", "--uid=0")
	for _, src := range srcs {
		args = append(args, "@"+src)
	}
	// Since the base tar and our do not have the uname set, this spews a lot of errors
	// of the form:
	// 	tar.exe: <file>: Can't translate uname '(null)' to UTF-8
	// So suppress errors the stderr feed
	_, err := sh.Exec(nil, nil, nil, "tar.exe", args...)
	return err
}

func convertTarToInitramfs(dst, src string) error {
	return sh.Run("tar", "-czf", dst, "--format=newc", "@"+src)
}
