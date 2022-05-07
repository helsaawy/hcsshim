//go:build windows

package main

import (
	"errors"

	"github.com/urfave/cli/v2"
)

var sampleCommand = &cli.Command{
	Name:    "sample",
	Aliases: []string{"s"},
	Usage:   "test exec changes",
	Action:  actionReExecWrapper(nil, sample),
}

func sample(c *cli.Context) error {
	return errors.New("no :(")
}
