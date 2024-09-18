// Tool to merge Linux rootfs.tar(.gz) and delta.tar (or other files) into
// a unified Linux rootfs TAR or CPIO archive.

package main

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"go.opencensus.io/trace"

	"github.com/Microsoft/hcsshim/internal/oc"
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	trace.ApplyConfig(trace.Config{DefaultSampler: oc.DefaultSampler})
	trace.RegisterExporter(&oc.LogrusExporter{})

	app := &cli.App{
		Name:  "lcowrootfs",
		Usage: "create Linux uVM root filesystem",

		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Aliases: []string{"lvl"},
				Usage:   "logging `level`",
				Value:   logrus.StandardLogger().Level.String(),
				Action: func(_ *cli.Context, s string) error {
					lvl, err := logrus.ParseLevel(s)
					if err == nil {
						logrus.SetLevel(lvl)
					}
					return err
				},
			},
		},

		Commands: []*cli.Command{
			merge,
			cpioCommand,
		},
		DefaultCommand: merge.Name,
		// Before: func(cCtx *cli.Context) error {
		// 	if !winapi.IsElevated() {
		// 		return fmt.Errorf(cCtx.App.Name + " must be run in an elevated context")
		// 	}

		// 	return nil
		// },
		ExitErrHandler: func(ctx *cli.Context, err error) {
			if err != nil {
				logrus.WithFields(logrus.Fields{
					logrus.ErrorKey: err,
					"command":       fmt.Sprintf("%#+v", os.Args),
				}).Error(ctx.App.Name + " failed")
			}
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(app.ErrWriter, err)
		os.Exit(1)
	}
}

const mergeOutputFlag = "output"

var merge = &cli.Command{
	Name:        "merge",
	Aliases:     []string{"m"},
	Usage:       "merge together multiple Linux layer tarballs",
	Description: "a combination of crane (github.com/google/go-containerregistry/cmd/crane) append and export commands",
	Args:        true,
	ArgsUsage:   "layers...",
	Flags: []cli.Flag{
		&cli.PathFlag{
			Name:     mergeOutputFlag,
			Aliases:  []string{"o"},
			Usage:    "output `path` for the merged tarball",
			Required: true,
		},
	},
	Action: func(cCtx *cli.Context) error {
		args := cCtx.Args()
		if args.Len() < 1 {
			return fmt.Errorf("no layers specified")
		}

		dest, err := filepath.Abs(cCtx.Path(mergeOutputFlag))
		if err != nil {
			return fmt.Errorf("invalid output file path %q: %w", cCtx.Path(mergeOutputFlag), err)
		}
		logrus.WithField("output", dest).Debug("using destination path")

		paths := make([]string, 0, args.Len())
		for _, s := range args.Slice() {
			p, err := filepath.Abs(s)
			if err != nil {
				return fmt.Errorf("invalid layer file path %q: %w", s, err)
			}
			paths = append(paths, p)
		}
		logrus.WithField("layers", paths).Debug("using layer paths")

		layers := make([]v1.Layer, 0, len(paths))
		for _, p := range paths {
			l, err := tarball.LayerFromFile(p, tarball.WithMediaType(types.OCILayer))
			if err != nil {
				return fmt.Errorf("create layer from %q: %w", p, err)
			}
			layers = append(layers, l)
		}

		base, err := baseImage()
		if err != nil {
			return fmt.Errorf("create base image: %w", err)
		}

		logrus.Trace("append layers to empty OCI image")
		img, err := mutate.AppendLayers(base, layers...)
		if err != nil {
			return fmt.Errorf("merge layers: %w", err)
		}

		w, err := os.Create(dest)
		if err != nil {
			return fmt.Errorf("create output file %q: %w", dest, err)
		}
		defer w.Close()

		if err := writeImage(w, img); err != nil {
			return fmt.Errorf("write merged layers to %q: %w", dest, err)
		}

		logrus.WithFields(logrus.Fields{
			"output": dest,
			"layers": paths,
		}).Info("merged layer tarball")
		return nil
	},
}

// writeTarToFile writes the tar stream to r, while undoing the changes that [mutate.Extract] made
// when it calls [filepath.Clean] on the file names (but not the link names):
//
//   - removes leading `./`
//   - removes trailing `/`
//   - replaces `\` with [os.Separator]
//
// Reapplying them standardizes the output with how tar creates the layers, makes the result cross-platform,
// and prevents broken (hard) links due to renamed files.
func writeImage(w io.WriteCloser, img v1.Image) error {
	logrus.Debug("update tar headers")

	r := mutate.Extract(img)
	defer r.Close()

	tr := tar.NewReader(r)

	tw := tar.NewWriter(w)
	defer tw.Close()

	for {
		header, err := tr.Next()
		switch {
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return fmt.Errorf("reading merged tar: %w", err)
		}

		entry := logrus.WithFields(logrus.Fields{
			"type":          header.FileInfo().Mode().String(),
			"original-name": header.Name,
		})

		header.Name = filepath.ToSlash(header.Name)
		if header.Typeflag == tar.TypeDir && !strings.HasSuffix(header.Name, `/`) {
			header.Name += `/`
		}
		if !strings.HasPrefix(header.Name, `./`) {
			header.Name = `./` + header.Name
		}
		entry.WithField("name", header.Name).Trace("updated file header")

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("write %q header: %w", header.Name, err)
		}
		if header.Size > 0 {
			if _, err := io.CopyN(tw, tr, header.Size); err != nil {
				return fmt.Errorf("write %q: %w", header.Name, err)
			}
		}
	}
}

// baseImage creates an configured base image to append layers to for consistent processing by crane
// (mostly to avoid the base being treated as a Windows image)
func baseImage() (v1.Image, error) {
	logrus.Debug("create base image")
	img := mutate.ConfigMediaType(mutate.MediaType(empty.Image, types.OCIManifestSchema1), types.OCIConfigJSON)

	cfg, err := img.ConfigFile()
	if err != nil { // shouldn't happen since its an empty image with nothing going on
		return nil, fmt.Errorf("compute config file: %w", err)
	}
	cfg.OS = "linux"
	cfg.Architecture = "amd64" // TODO: update this if we add ARM support
	if img, err = mutate.ConfigFile(img, cfg); err != nil {
		return nil, fmt.Errorf("update config file: %w", err)
	}
	return img, nil
}

var cpioCommand = &cli.Command{
	Name:  "cpio",
	Usage: "convert a tarball to a CPIO archive",
	Action: func(cCtx *cli.Context) error {
		// TODO
		return nil
	},
}
