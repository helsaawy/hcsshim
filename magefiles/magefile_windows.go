//go:build windows && mage

package main

import (
	"os"
	"path/filepath"
)

var (
	shellCmd         = filepath.Join(os.Getenv("SystemRoot"), `System32\cmd.exe`)
	shellFlags       = []string{"/e:on", "/c"}
	shellPipelineAnd = "&&"
)
