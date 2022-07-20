package ospath

import (
	"path"
	"path/filepath"

	"github.com/Microsoft/hcsshim/internal/os/name"
)

// Join joins paths using the target OS's path separator.
func Join(os name.OS, elem ...string) string {
	if os == name.Windows {
		return filepath.Join(elem...)
	}
	return path.Join(elem...)
}
