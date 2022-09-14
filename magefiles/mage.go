//go:build ignore

// This file is used in lieu of installing mage:
//  go run ./magefile/mage <target>

package main

import (
	"os"

	"github.com/magefile/mage/mage"
)

func main() { os.Exit(mage.Main()) }
