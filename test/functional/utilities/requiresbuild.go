package testutilities

import (
	"testing"

	osversion "github.com/Microsoft/hcsshim/internal/os/version"
)

func RequiresBuild(t *testing.T, b uint16) {
	if osversion.Build() < b {
		t.Skipf("Requires build %d+", b)
	}
}

func RequiresExactBuild(t *testing.T, b uint16) {
	if osversion.Build() != b {
		t.Skipf("Requires exact build %d", b)
	}
}
