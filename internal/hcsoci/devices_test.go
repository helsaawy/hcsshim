//go:build windows

package hcsoci

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Microsoft/hcsshim/pkg/annotations"
	"github.com/Microsoft/hcsshim/pkg/annotations/payload"
)

func TestParseKernelDrivers(t *testing.T) {
	driverDir1 := t.TempDir()
	driverDir2 := t.TempDir()
	driver1Path := filepath.Join(driverDir1, "fake.sys")
	driver2Path := filepath.Join(driverDir1, "also_fake.sys")
	driver3Path := filepath.Join(driverDir2, "stillfake.inf")
	vhd1Path := filepath.Join(driverDir1, "fake.vhdx")
	vhd2Path := filepath.Join(driverDir2, "fake.vhdx")

	for _, p := range []string{driver1Path, driver2Path, driver3Path, vhd1Path, vhd2Path} {
		f, err := os.Create(p)
		must(t, err, p)
		must(t, f.Close(), "closing "+p)
	}

	tests := []struct {
		name string
		got  string
		os   string
		want []*payload.Driver
		err  string
	}{
		// PnP
		{
			name: "Windows-List",
			got:  strings.Join([]string{driverDir1, driverDir2}, ","),
			os:   "windows",
			want: []*payload.Driver{
				{Path: driverDir1},
				{Path: driverDir2},
			},
		},
		{
			name: "Windows-JSON",
			got:  fmt.Sprintf(`[{"path":%q,"type":"Windows"}, {"path":%q}]`, driverDir1, driverDir2),
			os:   "windows",
			want: []*payload.Driver{
				{Path: driverDir1},
				{Path: driverDir2},
			},
		},
		{
			name: "Windows-List-error",
			got:  strings.Join([]string{driver2Path, driverDir2}, ","),
			os:   "windows",
			err:  fmt.Sprintf("invalid path for driver of type %v", payload.DriverTypeWindows),
		},
		{
			name: "Windows-JSON-error",
			got:  fmt.Sprintf(`[{"path":%q,"type":"Windows"}]`, driver2Path),
			os:   "windows",
			err:  fmt.Sprintf("invalid path for driver of type %v", payload.DriverTypeWindows),
		},

		// Legacy
		{
			name: "WindowsLegacy",
			got:  fmt.Sprintf(`[{"path":%q,"type":"WindowsLegacy"}, {"path":%q, "type": "WindowsLegacy"}]`, driver1Path, driver3Path),
			os:   "windows",
			want: []*payload.Driver{
				{
					Path: driver1Path,
					Type: payload.DriverTypeWindowsLegacy,
				},
				{
					Path: driver3Path,
					Type: payload.DriverTypeWindowsLegacy,
				},
			},
		},
		{
			name: "WindowsLegacy-error",
			got:  fmt.Sprintf(`[{"path":%q,"type":"WindowsLegacy"},{"path":%q,"type":"WindowsLegacy"}]`, driver3Path, driverDir1),
			os:   "windows",
			err:  fmt.Sprintf("invalid path for driver of type %v", payload.DriverTypeWindowsLegacy),
		},

		// Linux
		{
			name: "Linux-List",
			got:  strings.Join([]string{vhd1Path, vhd2Path}, ","),
			os:   "linux",
			want: []*payload.Driver{
				{
					Path: vhd1Path,
					Type: payload.DriverTypeLinux,
				},
				{
					Path: vhd2Path,
					Type: payload.DriverTypeLinux,
				},
			},
		},
		{
			name: "Linux-JSON",
			got:  fmt.Sprintf(`[{"path":%q,"type":"Linux"}, {"path":%q, "type": "Linux"}]`, vhd2Path, vhd1Path),
			os:   "linux",
			want: []*payload.Driver{
				{
					Path: vhd2Path,
					Type: payload.DriverTypeLinux,
				},
				{
					Path: vhd1Path,
					Type: payload.DriverTypeLinux,
				},
			},
		},
		{
			name: "Linux-List-error",
			got:  strings.Join([]string{vhd1Path, driver1Path}, ","),
			os:   "linux",
			err:  fmt.Sprintf("invalid path for driver of type %v", payload.DriverTypeLinux),
		},
		{
			name: "Linux-JSON-error",
			got:  fmt.Sprintf(`[{"path":%q,"type":"Linux"}]`, driverDir1),
			os:   "linux",
			err:  fmt.Sprintf("invalid path for driver of type %v", payload.DriverTypeLinux),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("string payload: %s", tc.got)
			m := map[string]string{annotations.VirtualMachineKernelDrivers: tc.got}

			drivers, err := getSpecKernelDrivers(m, tc.os)
			if tc.err == "" {
				t.Logf("%+v", tc.want)
				must(t, err, "could not parse payload")
				// no other way to check for equality than string encoding (for now ...)
				assert(t, reflect.DeepEqual(drivers, tc.want),
					fmt.Sprintf("got %+v, wanted %+v", drivers, tc.want))
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
