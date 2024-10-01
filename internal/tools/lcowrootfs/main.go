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
	"github.com/u-root/mkuimage/cpio"
	"github.com/u-root/mkuimage/uimage/initramfs"
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

// Note:
// file modification time when booting from initrd.img is the uVM boot time,
// since the kernel creates a rootfs (ramfs or tempfs) filesystem, instead of using the
// cpio archive directly.
//
// for rootfs.vhd, it is the original time as provided by the original tarball, since
// the VHD has an already-formatted (ext4) filesystem on it
//
// for the rootfs.vhd and intermediary tar and cpio archives, if they are created after
// extracting files to disk, the modification time for directories and symlinks will be
// the extraction date (and not the original date from the source).

const (
	mergeFlagOutput          = "output"
	mergeFlagNoTrailingSlash = "no-trailing-slash"
	mergeFlagNoOverrideOwner = "no-override-owner"
	mergeFlagConvertSlash    = "conver-slash"
)

var merge = &cli.Command{
	Name:    "merge",
	Aliases: []string{"m"},
	Usage:   "merge together multiple Linux layer tarballs",
	Description: strings.ReplaceAll(
		`merge layers without needing to extract and combine them on the file system.
This allows preserving file properties (e.g., creation date, owner user and group) which could
be changed by extraction`, "\n", " "),
	Args:      true,
	ArgsUsage: "layers...",
	Flags: []cli.Flag{
		&cli.PathFlag{
			Name:     mergeFlagOutput,
			Aliases:  []string{"o"},
			Usage:    "output tarball `file`",
			Required: true,
		},
		&cli.BoolFlag{
			Name:  mergeFlagNoTrailingSlash,
			Usage: "do not append a trailing slash (/) to directories",
		},
		&cli.BoolFlag{
			Name:  mergeFlagNoOverrideOwner,
			Usage: "do not set file owner UID and GID to 0",
		},
		&cli.BoolFlag{
			Name:  mergeFlagConvertSlash,
			Usage: "convert backslashes ('\\') in path names to slashes ('/')",
		},
	},
	// basically crane (github.com/google/go-containerregistry/cmd/crane) append and export
	Action: func(cCtx *cli.Context) error {
		args := cCtx.Args()
		if args.Len() < 1 {
			return fmt.Errorf("no layers specified")
		}

		dest, err := outputFilePath(cCtx.Path(mergeFlagOutput))
		if err != nil {
			return err
		}

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

		trailingSlash := !cCtx.Bool(mergeFlagNoTrailingSlash)
		overrideOwner := !cCtx.Bool(mergeFlagNoOverrideOwner)
		if err := writeImage(w, img, trailingSlash, overrideOwner); err != nil {
			return fmt.Errorf("write merged layers to %q: %w", dest, err)
		}

		logrus.WithFields(logrus.Fields{
			"output": dest,
			"layers": paths,
		}).Info("merged layer tarball")
		return nil
	},
}

// writeTarToFile writes the tar stream to r, while undoing some of the the changes that
// [mutate.Extract] made when it calls [filepath.Clean] on the file names (but not the link names):
//
//   - removes leading `./`
//   - removes trailing `/`
//   - replaces `\` with [os.Separator]
//
// We need to change `/` back to `\` on Windows to prevent broken (hard) links due to renamed files.
//
// Also, (GNU) tar and (BSD) tar.exe will prepend `./` depending on how the files are specified,
// but both will add a trailing slash to directories.
// Allow adding the trailing `/` to standardize with them.
//
// Also, allow overriding non-root (0) file ownershim, which may have been copied during tar creation.
func writeImage(w io.WriteCloser, img v1.Image, trailingSlash, overrideOwner bool) error {
	logrus.WithFields(logrus.Fields{
		"trailing-slash": trailingSlash,
		"override-owner": overrideOwner,
	}).Info("update tar headers")

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
			"directory": header.FileInfo().IsDir(),
			"name":      header.Name,
		})

		header.Name = filepath.ToSlash(header.Name)

		if trailingSlash && header.Typeflag == tar.TypeDir && !strings.HasSuffix(header.Name, `/`) {
			entry.Debug("append trailing slash to directory name")
			header.Name += `/`
		}

		// if !strings.HasPrefix(header.Name, `./`) {
		// 	header.Name = `./` + header.Name
		// }

		if overrideOwner && (header.Gid != 0 || header.Gname != "" || header.Uid != 0 || header.Uname != "") {
			entry.WithFields(logrus.Fields{
				"group":     header.Gid,
				"groupname": header.Gname,
				"user":      header.Uid,
				"username":  header.Uname,
			}).Debug("set user and group ownership to 0 (root)")

			header.Gid = 0
			header.Uid = 0
			header.Gname = ""
			header.Uname = ""
		}

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("write %q header: %w", header.Name, err)
		}
		if header.Size > 0 {
			if _, err := io.CopyN(tw, tr, header.Size); err != nil {
				return fmt.Errorf("write %q contents: %w", header.Name, err)
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

// current process (hack/catcpio.sh):
//  - extract: `cpio -iumd` (preseve modification date, create directories, overwrite) or `tar -xf`
//	- create: `cpio --create --format=newc -R 0:0`

const cpioOutputFlag = "output"

var cpioCommand = &cli.Command{
	Name:      "cpio",
	Aliases:   []string{"i", "initrd", "initramfs"},
	Usage:     "convert a layer tarball to an (uncompressed) newc-formatted CPIO archive",
	Args:      true,
	ArgsUsage: "layer",
	Flags: []cli.Flag{
		&cli.PathFlag{
			Name:     cpioOutputFlag,
			Aliases:  []string{"o"},
			Usage:    "output cpio archive `file`",
			Required: true,
		},
	},
	Action: func(cCtx *cli.Context) error {
		switch n := cCtx.NArg(); n {
		case 0:
			return fmt.Errorf("no layer specified")
		case 1:
		default:
			return fmt.Errorf("only one layer allowed (received %d)", n)
		}

		dest, err := outputFilePath(cCtx.Path(cpioOutputFlag))
		if err != nil {
			return err
		}

		layer, err := filepath.Abs(cCtx.Args().First())
		if err != nil {
			return fmt.Errorf("invalid layer file path %q: %w", cCtx.Args().First(), err)
		}
		logrus.WithField("layer", layer).Debug("using layer path")

		// TODO: tar reader
		// TODO: look at how hardlinks are set up
		// TODO: reject sparse TAR formats / file types
		// TODO: see if offset is reliable (f.Seek(io.SeekCurrent, 0)?)
		// TODO: does .ReadAt mess up the current offset?
		// TODO: does io.SectionReader do anything?
		// todo: set user:group owner to 0:0

		// "os".(*File).ReadAt should be safe for concurrent use, since, ultimately, the reads go to "internal/poll".(*FD).Pread
		// which is mutex locked and uses the specified offset

		if true {
			return nil
		}

		// u-root sorts the files by name first to make the creation reproducible
		// however, since we are loading from a tar (which doesn't really support random file access),
		// it doenst really make sense to do that.
		// instead, always have files added in the same order they are in from the layer tar.

		// cw, err := (&initramfs.CPIOFile{
		// 	Path: dest,
		// }).OpenWriter()
		// if err != nil {
		// 	return fmt.Errorf("create output file %q: %w", dest, err)
		// }

		// manually construct the records then add them, since we are creating it from a tar and
		// need to construct the info and handle hard links ourselves
		files := initramfs.NewFiles()
		// from tar files -> files.AddRecord()

		// don't use initramfs.Files, since:
		//  - it assumes files are located on disk, and polls that for information
		//  - it calls [cpio.MakeReproducible] on the files, which removes hardlinks

		// adhoc our own cpio.Recorder, where we incremend inode numbers, but still track hardlinks
		// we don't have any guarantees about file order, so the tar hardlink may be before
		// its target
		// save the hardlinks for later so we can add them all at the end, with the appropriate inode #
		//
		hardlinks := []cpio.Record{}

		fmt.Printf("hardlinks\n%#+v", hardlinks)

		// when reading tar, access the underlying tarball file to get offset of regular files, for use latter

		// todo: add in hardlinks

		// if hardlink, look up previous record and increment nlink
		//
		// symlink:
		//  	StaticRecord([]byte(linkname), info), nil
		// hardlink:
		//      same inode and reader?

		if err := initramfs.Write(&initramfs.Opts{
			Files: files,
			OutputFile: &initramfs.CPIOFile{
				Path: dest,
			},
		}); err != nil {
			return fmt.Errorf("write files to %q: %w", dest, err)
		}
		return nil
	},
}

// see 	"github.com/u-root/mkuimage/cpio".fs_windows.go
type tarRecorder struct {
	inumber uint64
}

func newTarRecorder() *tarRecorder { return &tarRecorder{inumber: 2} }

func (r *tarRecorder) inode() uint64 {
	r.inumber++
	return r.inumber - 1
}

// cCtx.Path(cpioOutputFlag))
func outputFilePath(s string) (string, error) {
	p, err := filepath.Abs(s)
	if err != nil {
		return "", fmt.Errorf("invalid output file path %q: %w", s, err)
	}

	entry := logrus.WithField("output", p)
	// if path is a (normal) file, it'll be silently overwritten
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		return "", fmt.Errorf("output file %q is a directory", p)
	} else if !os.IsNotExist(err) {
		// something weird happened, warn and hope the error goes away when we create it
		entry.WithError(err).Warn("unable to stat")
	}

	entry.Debug("using output path")
	return p, nil
}
