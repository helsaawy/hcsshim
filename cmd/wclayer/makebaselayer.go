//go:build windows

package main

import (
	"context"
	"path/filepath"

	"github.com/urfave/cli"

	"github.com/Microsoft/hcsshim"
	"github.com/Microsoft/hcsshim/computestorage"
	"github.com/Microsoft/hcsshim/internal/appargs"
)

var makeBaseLayerCommand = cli.Command{
	Name:      "makebaselayer",
	Usage:     "converts a directory containing 'Files/' into a base layer",
	ArgsUsage: "<layer path>",
	Before:    appargs.Validate(appargs.NonEmptyString),
	Action: func(context *cli.Context) error {
		path, err := filepath.Abs(context.Args().First())
		if err != nil {
			return err
		}

		return hcsshim.ConvertToBaseLayer(path)
	},
}

var processUVMImageCommand = cli.Command{
	Name:      "processuvmimage",
	Usage:     "update a utility VM base image by deleting and recreating files",
	ArgsUsage: "<base image path>",
	Before:    appargs.Validate(appargs.NonEmptyString),
	Action: func(cCtx *cli.Context) error {
		uvmPath, err := filepath.Abs(cCtx.Args().First())
		if err != nil {
			return err
		}

		// use computestorage since its newer and we need to switch anyways...
		return computestorage.SetupUtilityVMBaseLayer(context.Background(), uvmPath,
			filepath.Join(uvmPath, "SystemTemplateBase.vhdx"),
			filepath.Join(uvmPath, "SystemTemplate.vhdx"),
			20,
		)
	},
}
