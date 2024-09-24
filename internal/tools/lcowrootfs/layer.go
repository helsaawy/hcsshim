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
)

type layerMerge struct {
	// Append a `/` to directory names to be consistent with what GNU and BSD tar does.
	trailingSlash bool
	// Set file and directory owner user and group ID to 0 (root) and remove user and group name.
	overrideOwner bool
	// Override the tar header format.
	overrideTarFormat tar.Format
}

func newLayerMerge() *layerMerge {
	return &layerMerge{
		overrideTarFormat: tar.FormatPAX,
	}
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
	const whiteoutPrefix = ".wh."

	tarWriter := tar.NewWriter(w)
	defer tarWriter.Close()

	fileMap := map[string]bool{}

	// todo: slices.reverse

	// we iterate through the layers in reverse order because it makes handling
	// whiteout layers more efficient, since we can just keep track of the removed
	// files as we see .wh. layers and ignore those in previous layers.
	for i := len(layers) - 1; i >= 0; i-- {
		layer := layers[i]
		layerReader, err := uncompress(layer)
		if err != nil {
			return fmt.Errorf("uncompressing layer contents: %w", err)
		}

		tarReader := tar.NewReader(layerReader)
		for {
			header, err := tarReader.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return fmt.Errorf("reading tar: %w", err)
			}

		entry := logrus.WithFields(logrus.Fields{
			"directory": header.FileInfo().IsDir(),
			"name":      header.Name,
		})

			// use clean to remove leading 
			header.Name = path.Clean(header.Name)

		// header.Name = filepath.ToSlash(header.Name)

		if x.trailingSlash && header.Typeflag == tar.TypeDir && !strings.HasSuffix(header.Name, `/`) {
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
			if x.overrideTarFormat != tar.FormatUnknown {
				header.Format = x.overrideTarFormat
			}

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

			if _, ok := fileMap[name]; ok {
				continue
			}

			// check for a whited out parent directory
			if inWhiteoutDir(fileMap, name) {
				continue
			}

			// mark file as handled. non-directory implicitly tombstones
			// any entries with a matching (or child) name
			fileMap[name] = tombstone || !(header.Typeflag == tar.TypeDir)
			if !tombstone {
				if err := tarWriter.WriteHeader(header); err != nil {
					return err
				}
				if header.Size > 0 {
					if _, err := io.CopyN(tarWriter, tarReader, header.Size); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
func inWhiteoutDir(fileMap map[string]bool, file string) bool {
	for {
		if file == "" {
			break
		}
		dirname := path.Dir(file)
		if file == dirname {
			break
		}
		if val, ok := fileMap[dirname]; ok && val {
			return true
		}
		file = dirname
	}
	return false
}

// todo: move this into ./internal/tar and use in cmd/tar2ext4 and cmd/wclayer

// merge combines overlays layers/filesystems together.
var gzipHeader = []byte{0x1F, 0x8B, 8}

// uncompress detects if the reader is compressed, and, if so, returns an uncompressed reader.
//
// Adapted from [containerregistry PeekCompression], but with [io.ReaderAt] support and without zstd
// compression.
//
// [containerregistry extract]: https://github.com/google/go-containerregistry/blob/a07d1cab8700a9875699d2e7052f47acec30399d/internal/compression/compression.go#L52
func uncompress(r io.Reader) (io.Reader, error) {
	// a bufio.Reader
	type peeker interface { // TODO: name this something better...
		Peek(n int) ([]byte, error)
	}

	checkReadAt := func(r io.ReaderAt) func([]byte) (bool, error) {
		return func(header []byte) (bool, error) {
			b := make([]byte, len(header))
			if n, err := r.ReadAt(b, 0); err == io.EOF {
				return false, nil
			} else if err != nil {
				return false, err
			} else if n != len(header) {
				return false, fmt.Errorf("read %d returned %d bytes", len(header), n)
			}
			return bytes.Equal(b, header), nil
		}
	}
	checkPeek := func(r peeker) func([]byte) (bool, error) {
		return func(header []byte) (bool, error) {
			b, err := r.Peek(len(header))
			if err == io.EOF {
				return false, nil
			} else if err != nil {
				return false, err
			}
			return bytes.Equal(b, header), nil
		}
	}

	var checkHeader func([]byte) (bool, error)
	switch t := r.(type) {
	case io.ReaderAt:
		checkHeader = checkReadAt(t)
	case peeker:
		checkHeader = checkPeek(t)
	default:
		r = bufio.NewReader(r)
		checkHeader = checkPeek(r.(*bufio.Reader))
	}

	if ok, err := checkHeader(gzipHeader); err != nil {
		return nil, fmt.Errorf("check for gzip header: %w", err)
	} else if ok {
		return gzip.NewReader(r)
	}

	return r, nil
}
