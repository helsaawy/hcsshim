//go:build unix && mage

package main

var (
	shellCmd         = "/bin/sh"
	shellFlags       = []string{"-c"}
	shellPipelineAnd = "&&"
)
