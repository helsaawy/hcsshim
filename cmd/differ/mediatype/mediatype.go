// This package deals with media types and extensions specific to Windows containers (LCOW and WCOW).
package mediatype

import (
	"sort"
	"strings"
)

const (
	IsolatedMediaTypeExtension = "isolated"
	LCOWMediaTypeExtension     = "lcow"
	WCOWMediaTypeExtension     = "wcow"

	MediaTypeMicrosoftBase              = "application/vnd.microsoft"
	MediaTypeMicrosoftImageLayerVHD     = "application/vnd.microsoft.image.layer.v1.vhd"
	MediaTypeMicrosoftImageLayerExt4    = "application/vnd.microsoft.image.layer.v1.vhd+ext4"
	MediaTypeMicrosoftImageLayerWCLayer = "application/vnd.microsoft.image.layer.v1.vhd+wclayer"
)

func AddIsolatedExtension(mt string) string {
	return addExtension(mt, IsolatedMediaTypeExtension)
}

func AddLCOWExtension(mt string) string {
	return addExtension(mt, LCOWMediaTypeExtension)
}

func AddWCOWExtension(mt string) string {
	return addExtension(mt, WCOWMediaTypeExtension)
}

func addExtension(mt string, ext string) string {
	b, exts := parseMediaTypes(mt)
	for _, e := range exts {
		if e == ext {
			return mt
		}
	}
	exts = append(exts, ext)
	return unparseMediaTypes(b, exts)
}

// parseMediaTypes splits the media type into the base type and
// an array of extensions
//
// copied from github.com/containerd/containerd/images/mediatypes.go
func parseMediaTypes(mt string) (string, []string) {
	if mt == "" {
		return "", []string{}
	}

	s := strings.Split(mt, "+")
	ext := s[1:]

	return s[0], ext
}

// unparseMediaTypes joins together the base media type and the sorted extensions
func unparseMediaTypes(base string, ext []string) string {
	sort.Strings(ext)
	s := []string{base}
	s = append(s, ext...)

	return strings.Join(s, "+")
}
