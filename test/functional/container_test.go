//go:build windows && functional
// +build windows,functional

package functional

import (
	"context"
	"strings"
	"testing"

	ctrdoci "github.com/containerd/containerd/oci"
	"golang.org/x/sys/windows"

	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/Microsoft/hcsshim/osversion"

	"github.com/Microsoft/hcsshim/test/internal/cmd"
	"github.com/Microsoft/hcsshim/test/internal/container"
	"github.com/Microsoft/hcsshim/test/internal/layers"
	"github.com/Microsoft/hcsshim/test/internal/oci"
	"github.com/Microsoft/hcsshim/test/internal/util"
	"github.com/Microsoft/hcsshim/test/pkg/require"
	testuvm "github.com/Microsoft/hcsshim/test/pkg/uvm"
)

// debug: "The group or resource is not in the correct state to perform the requested operation" when removing vSMB from hyper-v

func Test_ContainerLifecycle(t *testing.T) {
	requireFeatures(t, featureContainer)
	requireAnyFeature(t, featureUVM, featureLCOW, featureWCOW)
	require.Build(t, osversion.RS5)

	ctx := namespacedContext()

	t.Run("LCOW", func(t *testing.T) {
		requireFeatures(t, featureLCOW, featureUVM)

		ls := linuxImageLayers(ctx, t)
		vm := testuvm.CreateAndStartLCOWFromOpts(ctx, t, defaultLCOWOptions(t))

		scratch, _ := layers.ScratchSpace(ctx, t, vm, "", "", "")
		cID := util.CleanName(t.Name()) + "_container"
		spec := oci.CreateLinuxSpec(ctx, t, cID,
			oci.DefaultLinuxSpecOpts("",
				ctrdoci.WithProcessArgs("/bin/sh", "-c", oci.TailNullArgs),
				oci.WithWindowsLayerFolders(append(ls, scratch)))...)

		c, _, cleanup := container.Create(ctx, t, vm, spec, cID, hcsOwner)
		t.Cleanup(cleanup)

		init := container.Start(ctx, t, c, nil)
		t.Cleanup(func() {
			container.Kill(ctx, t, c)
			container.Wait(ctx, t, c)
		})

		cmd.Kill(ctx, t, init)
		cmd.WaitExitCode(ctx, t, init, cmd.ForcedKilledExitCode)
	})

	t.Run("WCOW_HyperV", func(t *testing.T) {
		requireFeatures(t, featureWCOW, featureUVM)

		ls := windowsImageLayers(ctx, t)
		vm := testuvm.CreateAndStartWCOWFromOpts(ctx, t, defaultWCOWOptions(ctx, t))

		cID := util.CleanName(t.Name()) + "_container"
		scratch := layers.WCOWScratchDir(ctx, t, "")
		spec := oci.CreateWindowsSpec(ctx, t, cID,
			oci.DefaultWindowsSpecOpts("",
				ctrdoci.WithProcessCommandLine("cmd.exe /c ping -t 127.0.0.1"),
				oci.WithWindowsLayerFolders(append(ls, scratch)),
			)...)

		c, _, cleanup := container.Create(ctx, t, vm, spec, cID, hcsOwner)
		t.Cleanup(cleanup)

		init := container.StartWithSpec(ctx, t, c, spec.Process, nil)
		t.Cleanup(func() {
			container.Kill(ctx, t, c)
			container.Wait(ctx, t, c)
		})

		cmd.Kill(ctx, t, init)
		cmd.WaitExitCode(ctx, t, init, int(windows.ERROR_PROCESS_ABORTED))
	})

	t.Run("WCOW_Process", func(t *testing.T) {
		requireFeatures(t, featureWCOW)

		cID := util.CleanName(t.Name()) + "_container"
		scratch := layers.WCOWScratchDir(ctx, t, "")
		spec := oci.CreateWindowsSpec(ctx, t, cID,
			oci.DefaultWindowsSpecOpts("",
				ctrdoci.WithProcessCommandLine("cmd.exe /c ping -t 127.0.0.1"),
				oci.WithWindowsLayerFolders(append(windowsImageLayers(ctx, t), scratch)),
			)...)

		c, _, cleanup := container.Create(ctx, t, nil, spec, cID+"_Container", hcsOwner)
		t.Cleanup(cleanup)

		init := container.StartWithSpec(ctx, t, c, spec.Process, nil)
		t.Cleanup(func() {
			container.Kill(ctx, t, c)
			container.Wait(ctx, t, c)
		})

		cmd.Kill(ctx, t, init)
		cmd.WaitExitCode(ctx, t, init, int(windows.ERROR_PROCESS_ABORTED))
	})
}

type containerIOTest struct {
	name string
	args []string
	in   string
	want string
}

var ioTests = []containerIOTest{
	{
		name: "true",
		args: []string{"/bin/sh", "-c", "true"},
		want: "",
	},
	{
		name: "echo",
		args: []string{"/bin/sh", "-c", `echo -n "hi y'all"`},
		want: "hi y'all",
	},
	{
		name: "tee",
		args: []string{"/bin/sh", "-c", "tee"},
		in:   "are you copying me?",
		want: "are you copying me?",
	},
}

func TestLCOW_ContainerIO(t *testing.T) {
	requireFeatures(t, featureLCOW, featureContainer)
	require.Build(t, osversion.RS5)

	ctx := namespacedContext()

	for _, scenario := range []struct {
		name       string
		uvmCreator func(context.Context, *testing.T) *uvm.UtilityVM
		ioTestFunc func(context.Context, *testing.T, *uvm.UtilityVM, containerIOTest)
	}{
		{
			"LCOW",
			func(ctx context.Context, t *testing.T) *uvm.UtilityVM {
				opts := defaultLCOWOptions(t)
				return testuvm.CreateAndStartLCOWFromOpts(ctx, t, opts)
			},
			func(ctx context.Context, t *testing.T, vm *uvm.UtilityVM, tt containerIOTest) {
				ls := linuxImageLayers(ctx, t)
				cache := layers.CacheFile(ctx, t, "")
				id := util.CleanName(t.Name()) + "_container"
				scratch, _ := layers.ScratchSpace(ctx, t, vm, "", "", cache)
				spec := oci.CreateLinuxSpec(ctx, t, id,
					oci.DefaultLinuxSpecOpts(id,
						ctrdoci.WithProcessArgs(tt.args...),
						oci.WithWindowsLayerFolders(append(ls, scratch)))...)

				c, _, cleanup := container.Create(ctx, t, vm, spec, id, hcsOwner)
				t.Cleanup(cleanup)

				io := cmd.NewBufferedIO()
				if tt.in != "" {
					io = cmd.NewBufferedIOFromString(tt.in)
				}
				init := container.Start(ctx, t, c, io)

				t.Cleanup(func() {
					container.Kill(ctx, t, c)
					container.Wait(ctx, t, c)
				})

				if e := cmd.Wait(ctx, t, init); e != 0 {
					t.Fatalf("got exit code %d, wanted %d", e, 0)
				}

				io.TestOutput(t, tt.want, nil)
			},
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			vm := scenario.uvmCreator(ctx, t)
			for _, tt := range ioTests {
				t.Run(tt.name, func(t *testing.T) {
					scenario.ioTestFunc(ctx, t, vm, tt)
				})
			}
		})
	}

}

func TestLCOW_ContainerExec(t *testing.T) {
	requireFeatures(t, featureLCOW, featureContainer)
	require.Build(t, osversion.RS5)

	ctx := namespacedContext()
	ls := linuxImageLayers(ctx, t)
	opts := defaultLCOWOptions(t)
	opts.ID += util.RandNameSuffix()
	vm := testuvm.CreateAndStartLCOWFromOpts(ctx, t, opts)

	id := strings.ReplaceAll(t.Name(), "/", "") + util.RandNameSuffix()
	scratch, _ := layers.ScratchSpace(ctx, t, vm, "", "", "")
	spec := oci.CreateLinuxSpec(ctx, t, id,
		oci.DefaultLinuxSpecOpts(id,
			ctrdoci.WithProcessArgs("/bin/sh", "-c", oci.TailNullArgs),
			oci.WithWindowsLayerFolders(append(ls, scratch)))...)

	c, _, cleanup := container.Create(ctx, t, vm, spec, id, hcsOwner)
	t.Cleanup(cleanup)
	init := container.Start(ctx, t, c, nil)
	t.Cleanup(func() {
		cmd.Kill(ctx, t, init)
		cmd.Wait(ctx, t, init)
		container.Kill(ctx, t, c)
		container.Wait(ctx, t, c)
	})

	for _, tt := range ioTests {
		t.Run(tt.name, func(t *testing.T) {
			ps := oci.CreateLinuxSpec(ctx, t, id,
				oci.DefaultLinuxSpecOpts(id,
					// oci.WithTTY,
					ctrdoci.WithDefaultPathEnv,
					ctrdoci.WithProcessArgs(tt.args...))...,
			).Process
			io := cmd.NewBufferedIO()
			if tt.in != "" {
				io = cmd.NewBufferedIOFromString(tt.in)
			}
			p := cmd.Create(ctx, t, c, ps, io)
			cmd.Start(ctx, t, p)

			if e := cmd.Wait(ctx, t, p); e != 0 {
				t.Fatalf("got exit code %d, wanted %d", e, 0)
			}

			io.TestOutput(t, tt.want, nil)
		})
	}
}
