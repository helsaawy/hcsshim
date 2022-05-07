//go:build windows

package main

import (
	"fmt"
	"io"
	"os"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/Microsoft/hcsshim/cmd/differ/mediatype"
	"github.com/Microsoft/hcsshim/cmd/differ/payload"
	"github.com/Microsoft/hcsshim/ext4/tar2ext4"
)

var convertCommand = &cli.Command{
	Name:    "tar2ext4",
	Aliases: []string{"t2e4"},
	Usage: fmt.Sprintf("Convert an LCOW %q into an ext4 formated filesystem, %q",
		ocispec.MediaTypeImageLayer, mediatype.MediaTypeMicrosoftImageLayerExt4),
	Action: convertTarToExt4,
}

func convertTarToExt4(c *cli.Context) error {
	setupLogging()

	opts := &payload.Tar2Ext4Options{}
	if err := getPayload(c.Context, opts); err != nil {
		return err
	}

	logrus.Warningf("using options %+#v", opts)

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

	return nil
}
