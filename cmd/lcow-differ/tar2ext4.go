package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/typeurl"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli/v2"

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

	_, err := Tar2Ext4OptionsFromEnv(ctx)
	if err != nil {
		return err
	}
	return errors.New("no :(")

	dc, err := compression.DecompressStream(os.Stdin)
	if err != nil {
		return fmt.Errorf("decompress stream creation: %w", err)
	}
	if _, err = io.Copy(os.Stdout, dc); err != nil {
		return fmt.Errorf("io copy to std out: %w", err)
	}
	return nil
}

// need to be able to serialize tar2ext4 options across pipe
type Tar2Ext4Options struct {
	ConvertWhiteout bool
	AppendVhdFooter bool
	AppendDMVerity  bool
	InlineData      bool
	MaximumDiskSize int64
}

func Tar2Ext4OptionsFromEnv(ctx context.Context) (*Tar2Ext4Options, error) {
	b, err := getPayload(ctx)
	switch {
	case os.IsNotExist(err):
		return nil, nil
	case err != nil:
		return nil, err
	default:
	}

	var a types.Any
	if err := proto.Unmarshal(b, &a); err != nil {
		return nil, fmt.Errorf("could not proto.Unmarshal() decrypt data: %w", err)
	}
	v, err := typeurl.UnmarshalAny(&a)
	if err != nil {
		return nil, fmt.Errorf("unmarshal Tar2Ext4Options: %w", errdefs.ErrInvalidArgument)
	}

	o, ok := v.(*Tar2Ext4Options)
	if !ok {
		return nil, fmt.Errorf("payload type is %T, not Tar2Ext4Options: %w", v, errdefs.ErrInvalidArgument)
	}
	return o, nil
}

func (o *Tar2Ext4Options) Options() []tar2ext4.Option {
	opts := make([]tar2ext4.Option, 0, 5)
	if o == nil {
		return opts
	}

	if o.ConvertWhiteout {
		opts = append(opts, tar2ext4.ConvertWhiteout)
	}
	if o.AppendVhdFooter {
		opts = append(opts, tar2ext4.AppendVhdFooter)
	}
	if o.AppendDMVerity {
		opts = append(opts, tar2ext4.AppendDMVerity)
	}
	if o.InlineData {
		opts = append(opts, tar2ext4.InlineData)
	}
	if o.MaximumDiskSize != 0 {
		opts = append(opts, tar2ext4.MaximumDiskSize(o.MaximumDiskSize))
	}

	return opts
}
