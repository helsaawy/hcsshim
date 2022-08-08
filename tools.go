//go:build tools

package hcsshim

// Import packages used for go generate and other build steps so they are vendored and versioned
// with other dependencies.

import (
	_ "github.com/josephspurrier/goversioninfo/cmd/goversioninfo"
	_ "golang.org/x/tools/cmd/stringer"
)
