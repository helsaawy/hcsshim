//go:build windows

package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Microsoft/hcsshim/cmd/differ/mediatype"
	"github.com/Microsoft/hcsshim/cmd/differ/payload"
	"github.com/Microsoft/hcsshim/pkg/ociwclayer"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli/v2"
)

var wclayerCommand = &cli.Command{
	Name:    "wclayer",
	Aliases: []string{"wc"},
	Usage: fmt.Sprintf("Convert a %q stream and extract it into a Windows layer, %q",
		ocispec.MediaTypeImageLayer, mediatype.MediaTypeMicrosoftImageLayerVHD),
	Action: importFromTar,
}

func importFromTar(c *cli.Context) error {
	opts := &payload.WCLayerImportOptions{}
	if err := getPayload(c.Context, opts); err != nil {
		return err
	}
	if _, err := ociwclayer.ImportLayerFromTar(c.Context, os.Stdin, opts.RootPath, opts.Parents); err != nil {
		return err
	}
	// discard remaining data
	_, _ = io.Copy(io.Discard, os.Stdin)
	return nil
}
