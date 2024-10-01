package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/sirupsen/logrus"
)

type layerMerge struct {
	// Append a `/` to directory names to be consistent with what GNU and BSD tar does.
	TrailingSlash bool
	// Set file and directory owner user and group ID to 0 (root) and remove user and group name.
	OverrideOwner bool
	// Override the tar header format.
	OverrideTarFormat tar.Format
	// Replace `\` in path names with `/`.
	// Intended for tar file created on Windows, where `\` is the filepath separator.
	ConvertBackslash bool

	tw      *tar.Writer
	fileMap map[string]bool
}

func newLayerMerge(w io.Writer) *layerMerge {
	return &layerMerge{
		OverrideTarFormat: tar.FormatPAX,
		tw:                tar.NewWriter(w),
	}
}

func (x *layerMerge) close() error {
	tw := x.tw
	x.tw = nil
	clear(x.fileMap)
	return tw.Close()
}

// merge combines multiple tar filesystems (image layers) together, and writes the result to w.
//
// Adapted from [containerregistry extract], with several key differences:
//   - operate on layers directly, without needing intermediary image
//   - use [path] for path maniputlation instead of, so files are handled consistently on Windows and Linux
//   - allow overriding file ownership
//   - allow appending a trailing slash to directories
//
// [containerregistry extract]: https://github.com/google/go-containerregistry/blob/a07d1cab8700a9875699d2e7052f47acec30399d/pkg/v1/mutate/mutate.go#L264
func (x *layerMerge) merge(w io.Writer, layers ...io.Reader) error {
	if x.tw == nil || x.fileMap == nil || len(x.fileMap) != 0 {
		return fmt.Errorf("improperly created %T", x)
	}

	// todo: slices.reverse

	// we iterate through the layers in reverse order because it makes handling
	// whiteout layers more efficient, since we can just keep track of the removed
	// files as we see .wh. layers and ignore those in previous layers.
	for i := len(layers) - 1; i >= 0; i-- {
		layer := layers[i]
		if err := x.appendTo(layer); err != nil {
			return err
		}
	}
	return nil
}

// append the tar layer to w.
//
// based off of: https://github.com/google/go-containerregistry/blob/a07d1cab8700a9875699d2e7052f47acec30399d/pkg/v1/mutate/mutate.go#L264
func (x *layerMerge) appendTo(layer io.Reader) error {
	const whiteoutPrefix = ".wh."

	r, err := uncompress(layer)
	if err != nil {
		return fmt.Errorf("uncompressing layer contents: %w", err)
	}
	defer r.Close()

	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		header.Name = x.normalize(header.Name)

		entry := logrus.WithFields(logrus.Fields{
			"directory": header.FileInfo().IsDir(),
			"name":      header.Name,
		})

		basename := path.Base(header.Name)
		dirname := path.Dir(header.Name)
		tombstone := strings.HasPrefix(basename, whiteoutPrefix)
		if tombstone {
			basename = basename[len(whiteoutPrefix):]
		}

		// check if we have seen value before
		// if we're checking a directory, don't filepath.Join names
		var name string
		if header.Typeflag == tar.TypeDir {
			name = header.Name
		} else {
			name = path.Join(dirname, basename)
		}

		if _, ok := x.fileMap[name]; ok {
			continue
		}

		// check for a whited out parent directory
		if x.inWhiteoutDir(name) {
			continue
		}

		//
		// update header (as needed)
		//

		if x.TrailingSlash && header.Typeflag == tar.TypeDir && !strings.HasSuffix(header.Name, `/`) {
			entry.Debug("append trailing slash to directory name")
			header.Name += `/`
		}

		if x.OverrideOwner && (header.Gid != 0 || header.Gname != "" || header.Uid != 0 || header.Uname != "") {
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

		if x.OverrideTarFormat != tar.FormatUnknown && header.Format != x.OverrideTarFormat {
			entry.WithFields(logrus.Fields{
				"format":          header.Format.String(),
				"override-format": x.OverrideTarFormat.String(),
			}).Debug("override tar format")

			header.Format = x.OverrideTarFormat
		}

		// mark file as handled. non-directory implicitly tombstones
		// any entries with a matching (or child) name
		x.fileMap[name] = tombstone || !(header.Typeflag == tar.TypeDir)
		if !tombstone {
			if err := x.tw.WriteHeader(header); err != nil {
				return err
			}
			if header.Size > 0 {
				if _, err := io.CopyN(x.tw, tr, header.Size); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// based off of: https://github.com/google/go-containerregistry/blob/a07d1cab8700a9875699d2e7052f47acec30399d/pkg/v1/mutate/mutate.go#L264
func (x *layerMerge) inWhiteoutDir(file string) bool {
	for {
		if file == "" {
			break
		}
		dirname := path.Dir(file)
		if file == dirname {
			break
		}
		if val, ok := x.fileMap[dirname]; ok && val {
			return true
		}
		file = dirname
	}
	return false
}

// normalize names to avoid duplicates by calling [path.Clean], then removing leading slashes
func (x *layerMerge) normalize(p string) string {
	if x.ConvertBackslash {
		p = strings.ReplaceAll(p, `\`, "/")
	}
	return strings.TrimLeft(path.Clean(p), "/")
}

// todo: move this into ./internal/tar and use in cmd/tar2ext4 and cmd/wclayer

// uncompress detects if the reader is compressed, and, if so, returns an uncompressed reader.
//
// Adapted from [containerregistry PeekCompression], but with [io.ReaderAt] support.
//
// [containerregistry PeekCompression]: https://github.com/google/go-containerregistry/blob/a07d1cab8700a9875699d2e7052f47acec30399d/internal/compression/compression.go#L52
func uncompress(r io.Reader) (io.ReadCloser, error) {
	var gzipHeader = []byte{0x1F, 0x8B, 8}

	r, chkFn := getCheckHeaderFn(r)

	// layers can be tar+gzip or tar+zstd
	// TODO: add zstd support
	if ok, err := chkFn(gzipHeader); err != nil {
		return nil, fmt.Errorf("check for gzip header: %w", err)
	} else if ok {
		return gzip.NewReader(r)
	}

	return io.NopCloser(r), nil
}

// checkHeaderFn checks if the header was found in the underlying reader.
// it does not modify the reader's current state.
type checkHeaderFn func(header []byte) (bool, error)

// based off of: https://github.com/google/go-containerregistry/blob/a07d1cab8700a9875699d2e7052f47acec30399d/internal/compression/compression.go#L52
func getCheckHeaderFn(r io.Reader) (io.Reader, checkHeaderFn) {
	type peekReader interface {
		io.Reader
		Peek(n int) ([]byte, error)
	}

	var p peekReader
	switch rr := r.(type) {
	case io.ReaderAt:
		fn := func(header []byte) (bool, error) {
			b := make([]byte, len(header))
			if n, err := rr.ReadAt(b, 0); err == io.EOF {
				return false, nil
			} else if err != nil {
				return false, err
			} else if n != len(header) {
				return false, fmt.Errorf("read %d returned %d bytes", len(header), n)
			}
			return bytes.Equal(b, header), nil
		}
		return r, fn
	case peekReader:
		p = rr
	default:
		p = bufio.NewReader(r)
	}

	fn := func(header []byte) (bool, error) {
		b, err := p.Peek(len(header))
		if err == io.EOF {
			return false, nil
		} else if err != nil {
			return false, err
		}
		return bytes.Equal(b, header), nil
	}
	return r, fn
}
