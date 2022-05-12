//go:build windows

package main

import (
	"errors"

	"github.com/Microsoft/go-winio"
	"github.com/urfave/cli/v2"
)

var sampleCommand = &cli.Command{
	Name:    "sample",
	Aliases: []string{"s"},
	Usage:   "test exec changes",
	Action:  actionReExecWrapper(sample, withPrivileges([]string{winio.SeBackupPrivilege, winio.SeRestorePrivilege})),
}

func sample(c *cli.Context) error {
	return errors.New("no :(")
}
