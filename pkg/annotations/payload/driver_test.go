package payload

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDriverTypeJSON(t *testing.T) {
	for s, dt := range driverTypeValues {
		t.Run(s, func(t *testing.T) {
			s = `"` + s + `"`

			var v DriverType
			err := json.Unmarshal([]byte(s), &v)
			must(t, err, "could not unmarshal")
			assert(t, v == dt, fmt.Sprintf("got %v, wanted %v)", v, dt))

			b, err := json.Marshal(v)
			must(t, err, "could not marshal")
			assert(t, string(b) == s, fmt.Sprintf("got %q, wanted %q)", b, s))
		})
	}
}

func TestDriversNonExistant(t *testing.T) {
	for s, dt := range driverTypeValues {
		t.Run(s, func(t *testing.T) {
			d := Driver{
				Path: `G:\this\path\better\not\exist`,
				Type: dt,
			}
			is(t, d.Validate(), os.ErrNotExist)
		})
	}
}

func TestDrivers(t *testing.T) {
	driverDir := t.TempDir()
	driverPath := filepath.Join(driverDir, "fake.sys")
	vhdPath := filepath.Join(driverDir, "fake.vhdx")
	for _, p := range []string{driverPath, vhdPath} {
		f, err := os.Create(p)
		must(t, err, p)
		must(t, f.Close(), "closing "+p)
	}

	tests := []struct {
		name   string
		driver *Driver
		err    string
	}{
		{
			name:   "PnP",
			driver: &Driver{Path: driverDir, Type: DriverTypeWindows},
			err:    "",
		},
		{
			name:   "PnP-file",
			driver: &Driver{Path: driverPath, Type: DriverTypeWindows},
			err:    fmt.Sprintf("invalid path for driver of type %v", DriverTypeWindows),
		},
		{
			name:   "legacy",
			driver: &Driver{Path: driverPath, Type: DriverTypeWindowsLegacy},
			err:    "",
		},
		{
			name:   "legacy-dir",
			driver: &Driver{Path: driverDir, Type: DriverTypeWindowsLegacy},
			err:    fmt.Sprintf("invalid path for driver of type %v", DriverTypeWindowsLegacy),
		},
		{
			name:   "Linux",
			driver: &Driver{Path: vhdPath, Type: DriverTypeLinux},
			err:    "",
		},
		{
			name:   "Linux-dir",
			driver: &Driver{Path: driverDir, Type: DriverTypeLinux},
			err:    fmt.Sprintf("invalid path for driver of type %v", DriverTypeLinux),
		},
		{
			name:   "Linux-not-vhd",
			driver: &Driver{Path: driverPath, Type: DriverTypeLinux},
			err:    fmt.Sprintf("invalid path for driver of type %v", DriverTypeLinux),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := json.Marshal(tc.driver)
			t.Logf("%s", s)
			err := tc.driver.Validate()
			if tc.err == "" {
				must(t, err, "validate failed")
			} else {
				assert(t, err != nil && strings.Contains(err.Error(), tc.err),
					fmt.Sprintf("err %v does not container %q", err, tc.err))
			}
		})
	}
}

func assert(t testing.TB, b bool, msgs ...string) {
	if b {
		return
	}
	t.Helper()
	t.Fatalf(msgJoin(msgs, "failed assertion"))
}

func is(t testing.TB, err, target error, msgs ...string) {
	if errors.Is(err, target) {
		return
	}
	t.Helper()
	t.Fatalf(msgJoin(msgs, "got error %q; wanted %q"), err, target)
}

func must(t testing.TB, err error, msgs ...string) {
	if err == nil {
		return
	}
	t.Helper()
	t.Fatalf(msgJoin(msgs, "%v"), err)
}

func msgJoin(pre []string, s string) string {
	return strings.Join(append(pre, s), ": ")
}
