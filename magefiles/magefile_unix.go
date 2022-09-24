//go:build unix && mage

package main

import "errors"

//nolint:unused,varcheck
var (
	shellCmd         = "/bin/sh"
	shellFlags       = []string{"-c"}
	shellPipelineAnd = "&&"
)

func mergeTarFiles(dst string, srcs ...string) error {
	return errors.New("not implemented yet :/") //TODO
}

func convertTarToInitramfs(dst, src string) error {
	return errors.New("not implemented yet :/") //TODO
}
