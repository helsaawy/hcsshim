//go:build windows && functional

package functional

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	ctrdoci "github.com/containerd/containerd/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/windows"

	"github.com/Microsoft/hcsshim/internal/memory"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/Microsoft/hcsshim/internal/winapi"
	"github.com/Microsoft/hcsshim/osversion"

	testcmd "github.com/Microsoft/hcsshim/test/internal/cmd"
	testoci "github.com/Microsoft/hcsshim/test/internal/oci"
	"github.com/Microsoft/hcsshim/test/internal/util"
	"github.com/Microsoft/hcsshim/test/pkg/require"
	testuvm "github.com/Microsoft/hcsshim/test/pkg/uvm"
)

func TestUVMMemoryUpdate(t *testing.T) {
	requireFeatures(t, featureUVM)
	requireAnyFeature(t, featureLCOW, featureWCOW)
	require.Build(t, osversion.RS5)

	const startingMemSize uint64 = 2 * memory.GiB
	ctx := util.Context(namespacedContext(context.Background()), t)

	type config struct {
		feature    string
		createOpts func(context.Context, testing.TB) any
		memSize    uint64
		updateErr  error
	}

	configs := make([]config, 0)
	for _, tc := range []config{
		{
			memSize: startingMemSize / 2,
		},
		{
			memSize: startingMemSize,
		},
		{
			memSize: 2 * startingMemSize,
		},
		{
			memSize:   2 * memory.MiB,
			updateErr: windows.Errno(0x8004102b), // WBEM_E_VALUE_OUT_OF_RANGE: 0x8004102b / int32(-2147217365)
		},
	} {
		for _, c := range []struct {
			feature    string
			createOpts func(context.Context, testing.TB) any
		}{
			{
				feature: featureLCOW,
				//nolint: thelper
				createOpts: func(_ context.Context, tb testing.TB) any { return defaultLCOWOptions(ctx, tb) },
			},
			{
				feature: featureWCOW,
				//nolint: thelper
				createOpts: func(ctx context.Context, tb testing.TB) any { return defaultWCOWOptions(ctx, tb) },
			},
		} {
			conf := tc
			conf.feature = c.feature
			conf.createOpts = c.createOpts
			configs = append(configs, conf)
		}
	}

	verifyMemoryInVM := func(ctx context.Context, tb testing.TB, vm *uvm.UtilityVM, memSize uint64) {
		tb.Helper()

		var ps *specs.Process
		var searchStr string
		if vm.OS() == "windows" {
			searchStr = "PhysicallyInstalledSystemMemory " + strconv.FormatUint(memSize, 10)
			reexecCmd := fmt.Sprintf(`%s -test.run=%s`, filepath.Join(`C:\`, filepath.Base(os.Args[0])), util.TestNameRegex(tb))
			if testing.Verbose() {
				reexecCmd += " -test.v"
			}

			ps = testoci.CreateWindowsSpec(ctx, tb, vm.ID(),
				testoci.DefaultWindowsSpecOpts(vm.ID(),
					ctrdoci.WithUsername(`NT AUTHORITY\SYSTEM`),
					ctrdoci.WithEnv([]string{util.ReExecEnv + "=1"}),
					ctrdoci.WithProcessCommandLine(reexecCmd),
				)...).Process
		} else {
			searchStr = strconv.FormatUint(memSize/memory.KiB, 10) + " kB"
			ps = testoci.CreateLinuxSpec(ctx, tb, vm.ID(),
				testoci.DefaultLinuxSpecOpts(vm.ID(),
					ctrdoci.WithDefaultPathEnv,
					ctrdoci.WithProcessArgs("/bin/sh", "-c", "vmstat -s"),
					// ctrdoci.WithProcessArgs("/bin/sh", "-c", "cat /proc/meminfo | grep -i 'MemTotal'"),
				)...,
			).Process
		}

		io := testcmd.NewBufferedIO()
		c := testcmd.Create(ctx, tb, vm, ps, io)
		testcmd.Start(ctx, tb, c)
		testcmd.WaitExitCode(ctx, tb, c, 0)
		io.TestStdOutContains(tb, []string{searchStr}, nil)
	}

	for _, tc := range configs {
		t.Run(fmt.Sprintf("%s %d", tc.feature, tc.memSize), func(t *testing.T) {
			requireFeatures(t, tc.feature)

			// get total memory available from inside guest
			// will only be called for WCOW - other way is to deal with WMI commands ...
			util.RunInReExec(ctx, t, func(ctx context.Context, t testing.TB) {
				kb, err := winapi.GetPhysicallyInstalledSystemMemory()
				if err != nil {
					t.Fatalf("failed to get installed memory: %v", err)
				}
				fmt.Println("PhysicallyInstalledSystemMemory", kb*memory.KiB)
			})

			var newMemSize = tc.memSize // copy value to avoid weird for-loop aliasing issues when taking address

			opts := tc.createOpts(ctx, t)
			switch opts := opts.(type) {
			case *uvm.OptionsWCOW:
				opts.MemorySizeInMB = startingMemSize / memory.MiB
				opts.AllowOvercommit = false
				opts.EnableDeferredCommit = false
				opts.FullyPhysicallyBacked = true
			case *uvm.OptionsLCOW:
				opts.MemorySizeInMB = startingMemSize / memory.MiB
				opts.AllowOvercommit = false
				opts.EnableDeferredCommit = false
				opts.FullyPhysicallyBacked = true
			default:
				t.Fatalf("unknown options type %T", opts)
			}
			vm := testuvm.CreateAndStart(ctx, t, opts)

			if m, err := vm.GetAssignedMemoryInBytes(ctx); err != nil {
				t.Fatalf("failed to get UVM memory: %v", err)
			} else if m != startingMemSize {
				t.Fatalf("expected memory side %d, got %d", startingMemSize, m)
			}

			if vm.OS() == "windows" {
				testuvm.Share(ctx, t, vm, os.Args[0], filepath.Join(`C:\`, filepath.Base(os.Args[0])), true)
			}
			verifyMemoryInVM(ctx, t, vm, startingMemSize)

			// WindowsResources works both for WCOW and LCOW
			err := vm.Update(ctx, &specs.WindowsResources{
				Memory: &specs.WindowsMemoryResources{
					Limit: &newMemSize,
				},
			}, nil)

			time.Sleep(1000 * time.Millisecond) // wait a bit for update things

			if !errors.Is(err, tc.updateErr) {
				t.Fatalf("expected update error %v; got: %v", tc.updateErr, err)
			}
			if err != nil || tc.updateErr != nil {
				return
			}

			// validate update from host side
			if m, err := vm.GetAssignedMemoryInBytes(ctx); err != nil {
				t.Fatalf("failed to get UVM memory: %v", err)
			} else if m != newMemSize {
				t.Fatalf("expected memory side %d, got %d", newMemSize, m)
			}

			// validate update from guest side
			verifyMemoryInVM(ctx, t, vm, newMemSize)
		})
	}
}
