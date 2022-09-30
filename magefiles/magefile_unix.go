//go:build unix && mage

package main

import "errors"

//nolint:unused,varcheck
const (
	shellCmd         = "/bin/sh"
	shellPipelineAnd = "&&"

	archiveExt = ".tar.gz"
	binaryExt  = "" // replacement for GOEXE
)

//nolint:unused,varcheck
var (
	shellFlags = []string{"-c"}
)

func mergeTarFiles(dst string, srcs ...string) error {
	return errors.New("not implemented yet :(") //TODO
}

func convertTarToInitramfs(dst, src string) error {
	return errors.New("not implemented yet :(") //TODO
}
