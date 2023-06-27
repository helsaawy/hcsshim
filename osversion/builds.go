package osversion

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
)

var ErrUnsupportedBuild = errors.New("unsupported build")

type version struct {
	version   string
	ltsc      string
	container bool
}

var ( // leading `_` to avoid accidental overwriting in code
	_builds = map[uint16]version{
		RS1: { // Redstone 1
			version: "1607",
			ltsc:    "2016",
		},
		RS2: { // Redstone 2
			version: "1703",
		},

		RS3: { // Redstone 3
			version:   "1709",
			container: true,
		},

		RS4: { // Redstone 4
			version:   "1803",
			container: true,
		},

		RS5: { // Redstone 5
			version:   "1809",
			ltsc:      "2019",
			container: true,
		},

		V19H1: {
			version:   "1903",
			container: true,
		},

		V19H2: { // Vanadium
			version:   "1909",
			container: true,
		},

		V20H1: { // Vibranium
			version:   "2004",
			container: true,
		},

		V20H2: { // Vibranium v2
			version:   "20H2",
			container: true,
		},

		V21H1: { // Vibranium v3
			version: "21H1",
		},

		V21H2Server: {
			version:   "21H2",
			ltsc:      "2022",
			container: true,
		},

		// windows 10

		V21H2Win10: { // Vibranium v4
			version: "21H2",
		},
		V22H2Win10: { // Vibranium v5
			version: "22H2",
		},

		// windows 11

		V21H2Win11: { // Sun Valley 1
			version: "21H2",
		},
		V22H2Win11: { // Sun Valley 2
			version: "22H2",
		},
	}
)

func ContainerBuilds() []uint16 {
	bs := make([]uint16, 0, len(_builds))
	for b, v := range _builds {
		if v.container {
			bs = append(bs, b)
		}
	}
	slices.Sort(bs)
	return bs
}

func ltscBuilds() []uint16 {
	bs := make([]uint16, 0, len(_builds))
	for b, v := range _builds {
		if v.container && v.ltsc != "" {
			bs = append(bs, b)
		}
	}
	slices.Sort(bs)
	return bs
}

// ContainerTagFromBuild returns the appropriate "mcr.microsoft.com/windows/[nanoserver|servercore|server]" tag
// for a given build, or an error if an appropriate tag doesn't exist.
// The tag may not exist for all Windows container base images.
//
// Takes into consideration ABI stability post LTSC 2022.
//
// See:
// https://learn.microsoft.com/en-us/virtualization/windowscontainers/manage-containers/container-base-images
// https://learn.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/base-image-lifecycle
func ContainerTagFromBuild(b uint16) (string, error) {
	v, ok := _builds[b]
	if !ok || !v.container { // no exact tag found
		if b < LTSC2022 {
			// no ABI stability guarantee
			return "", fmt.Errorf("no container tag for build %d: %w", b, ErrUnsupportedBuild)
		}

		// find latest ltsc version less than the current build
		for _, ltsc := range ltscBuilds() {
			if ltsc <= b {
				v = _builds[ltsc] // build should be in the map, otherwise ...
			}
		}
	}

	if v.ltsc != "" {
		return strings.ToLower("ltsc" + v.ltsc), nil
	}

	return strings.ToLower(v.version), nil
}
