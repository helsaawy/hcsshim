//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli/v2"

	"github.com/Microsoft/hcsshim/cmd/differ/opts"
	"github.com/Microsoft/hcsshim/ext4/tar2ext4"
)

var convertCommand = &cli.Command{
	Name:    "convert",
	Aliases: []string{"conv", "c"},
	Usage:   fmt.Sprintf("convert a tar %q layer into an ext4 formated filesystem", ocispec.MediaTypeImageLayer),
	Action:  convert,
}

func convert(c *cli.Context) error {
	ctx := c.Context
	f, err := os.Create("C:\\t\\t.txt")
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck

	opts, err := Tar2Ext4OptionsFromEnv(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(f, "%#+v\n", opts)

	vhd, err := os.Create(opts.VHDPath)
	if err != nil {
		return err
	}
	defer vhd.Close()
	if err = tar2ext4.Convert(os.Stdin, vhd, opts.Options()...); err != nil {
		return err
	}
	if err = vhd.Sync(); err != nil {
		return err
	}

	// discard remaining data
	_, _ = io.Copy(io.Discard, os.Stdin)

	return errors.New("sad face :(")
}

func Tar2Ext4OptionsFromEnv(ctx context.Context) (*opts.Tar2Ext4Options, error) {
	b, err := getPayload(ctx)
	switch {
	case os.IsNotExist(err):
		return nil, nil
	case err != nil:
		return nil, err
	default:
	}

	a := &types.Any{}
	if err := proto.Unmarshal(b, a); err != nil {
		return nil, fmt.Errorf("proto.Unmarshal() on Tar2Ext4Options: %w", err)
	}
	o := &opts.Tar2Ext4Options{}
	err = o.FromAny(a)
	return o, err
}
