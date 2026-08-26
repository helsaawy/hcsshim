//go:build windows && functional
// +build windows,functional

package functional

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ctrdoci "github.com/containerd/containerd/v2/pkg/oci"

	"github.com/Microsoft/hcsshim/hcn"
	"github.com/Microsoft/hcsshim/internal/hns"
	"github.com/Microsoft/hcsshim/osversion"

	testcmd "github.com/Microsoft/hcsshim/test/internal/cmd"
	testcontainer "github.com/Microsoft/hcsshim/test/internal/container"
	testlayers "github.com/Microsoft/hcsshim/test/internal/layers"
	testoci "github.com/Microsoft/hcsshim/test/internal/oci"
	"github.com/Microsoft/hcsshim/test/internal/util"
	"github.com/Microsoft/hcsshim/test/pkg/require"
	testuvm "github.com/Microsoft/hcsshim/test/pkg/uvm"
)

func TestLCOW_RepeatedEndpointAdd(t *testing.T) {
	requireFeatures(t, featureLCOW, featureUVM)
	require.Build(t, osversion.RS5)

	ns, err := newNetworkNamespace()
	if err != nil {
		t.Fatalf("namespace creation: %v", err)
	}
	t.Cleanup(func() {
		if err := ns.Delete(); err != nil {
			t.Errorf("namespace delete: %v", err)
		}
	})
	t.Logf("created namespace %s", ns.Id)

	ipRoute := hcn.Route{
		NextHop:           "192.168.128.1",
		DestinationPrefix: "0.0.0.0/0",
	}

	ipWant := hcn.IpConfig{
		IpAddress:    "192.168.128.4",
		PrefixLength: 20,
	}

	// create network and endpoint
	ntwk, err := (&hcn.HostComputeNetwork{
		Name: hcsOwner + "network",
		Type: hcn.NAT,
		Ipams: []hcn.Ipam{
			{
				Type: "Static",
				Subnets: []hcn.Subnet{
					{
						IpAddressPrefix: "192.168.128.0/20",
						Routes:          []hcn.Route{ipRoute},
					},
				},
			},
		},
		SchemaVersion: hcn.Version{Major: 2, Minor: 2},
	}).Create()
	if err != nil {
		t.Fatalf("network creation: %v", err)
	}
	t.Cleanup(func() {
		if err := ntwk.Delete(); err != nil {
			t.Errorf("network delete: %v", err)
		}
	})
	t.Logf("created network %s (%s)", ntwk.Name, ntwk.Id)

	ep, err := (&hcn.HostComputeEndpoint{
		Name:               ntwk.Name + "endpoint",
		HostComputeNetwork: ntwk.Id,
		Routes:             []hcn.Route{ipRoute},
		IpConfigurations:   []hcn.IpConfig{ipWant},
		SchemaVersion:      ntwk.SchemaVersion,
	}).Create()
	if err != nil {
		t.Fatalf("endpoint creation: %v", err)
	}
	t.Cleanup(func() {
		if err := ep.Delete(); err != nil {
			t.Errorf("endpoint delete: %v", err)
		}
	})
	t.Logf("created endpoint %s", ep.Id)

	// uVM methods want the HNS types, not the HCN
	// should only need the ID, but get the full object just to be safe
	hnsEP, err := hns.GetHNSEndpointByID(ep.Id)
	if err != nil {
		t.Fatalf("get HNS endpoint: %v", err)
	}

	for _, ip := range ep.IpConfigurations {
		if ip != ipWant {
			t.Fatalf("endpoint address %v != %v", ip, ipWant)
		}
		t.Logf("ip %s/%d", ip.IpAddress, ip.PrefixLength)
	}

	if err := ep.NamespaceAttach(ns.Id); err != nil {
		t.Fatalf("network attachment: %v", err)
	}

	ctx := namespacedContext(t.Context())
	opts := defaultLCOWOptions(ctx, t)
	opts.ExecCommandLine += " -kmsgLogLevel=7 "
	opts.KernelBootOptions += "spec_rstack_overflow=off"
	opts.VPMemDeviceCount = 0
	opts.VPCIEnabled = true
	opts.FullyPhysicallyBacked = true
	opts.AllowOvercommit = false

	vm := testuvm.CreateAndStartLCOWFromOpts(ctx, t, opts)

	// set up tracing events
	// switch to the root cgroup first, then enable the required events
	func() {
		ps := testoci.CreateLinuxSpec(ctx, t, vm.ID(),
			testoci.DefaultLinuxSpecOpts(ns.Id,
				ctrdoci.WithDefaultPathEnv,
				ctrdoci.WithProcessArgs("/bin/sh", "-c", `echo 0 > /sys/fs/cgroup/cgroup.procs
mount -t tracefs nodev /sys/kernel/tracing

echo hyperv:vmbus_on_msg_dpc >> /sys/kernel/tracing/set_event
echo hyperv:vmbus_on_message >> /sys/kernel/tracing/set_event
echo hyperv:vmbus_onoffer >> /sys/kernel/tracing/set_event
echo hyperv:vmbus_onoffer_rescind >> /sys/kernel/tracing/set_event
echo hyperv:vmbus_onopen_result >> /sys/kernel/tracing/set_event
echo hyperv:vmbus_ongpadl_created >> /sys/kernel/tracing/set_event
echo hyperv:vmbus_ongpadl_torndown >> /sys/kernel/tracing/set_event
echo hyperv:vmbus_open >> /sys/kernel/tracing/set_event
echo hyperv:vmbus_close_internal >> /sys/kernel/tracing/set_event
echo hyperv:vmbus_establish_gpadl_header >> /sys/kernel/tracing/set_event
echo hyperv:vmbus_establish_gpadl_body >> /sys/kernel/tracing/set_event
echo hyperv:vmbus_teardown_gpadl >> /sys/kernel/tracing/set_event
echo hyperv:vmbus_release_relid >> /sys/kernel/tracing/set_event
echo hyperv:vmbus_send_tl_connect_request >> /sys/kernel/tracing/set_event
`),
			)...,
		).Process
		io := testcmd.NewBufferedIO()
		p := testcmd.Create(ctx, t, vm, ps, io)
		testcmd.Start(ctx, t, p)

		e := testcmd.Wait(ctx, t, p)
		out, err := io.Output()
		t.Logf("enabling trace events output:\n%s", out)
		if e != 0 || err != nil {
			t.Fatalf("enabling tracing failed with exit code %d and error %v", e, err)
		}
	}()

	func() {
		ps := testoci.CreateLinuxSpec(ctx, t, vm.ID(),
			testoci.DefaultLinuxSpecOpts(ns.Id,
				ctrdoci.WithDefaultPathEnv,
				ctrdoci.WithProcessArgs("/bin/sh", "-c", `cat /sys/kernel/tracing/set_event`),
			)...,
		).Process
		io := testcmd.NewBufferedIO()
		p := testcmd.Create(ctx, t, vm, ps, io)
		testcmd.Start(ctx, t, p)

		e := testcmd.Wait(ctx, t, p)
		out, err := io.Output()
		t.Logf("set_event file output:\n%s", out)
		if e != 0 || err != nil {
			t.Fatalf("get tracing events failed with exit code %d and error %v", e, err)
		}
	}()

	logDir := filepath.Dir(util.TestingBinaryPath(ctx, t))

	// pipe the tracing events into a file
	go func() {
		// keep running until the vm is removed
		ctx := context.WithoutCancel(ctx)
		s := filepath.Join(logDir, "trace"+util.RandNameSuffix(t.Name())+".log")
		t.Logf("writing trace data to: %s", s)
		f, err := os.Create(s)
		if err != nil {
			t.Errorf("failed to open trace file for writing: %v", err)
			return
		}
		defer f.Close()

		ps := testoci.CreateLinuxSpec(ctx, t, vm.ID(),
			testoci.DefaultLinuxSpecOpts(ns.Id,
				ctrdoci.WithDefaultPathEnv,
				ctrdoci.WithProcessArgs("/bin/sh", "-c", `tail +1 -f /sys/kernel/tracing/trace`),
			)...,
		).Process
		p := testcmd.Create(ctx, t, vm, ps, nil)
		p.Stderr = f
		p.Stdout = f

		testcmd.Start(ctx, t, p)

		err = p.Wait()
		t.Logf("[%s] get tracing events ended: %v", time.Now().Format(time.RFC3339Nano), err)
	}()

	go func() {
		// keep running until the vm is removed
		ctx := context.WithoutCancel(ctx)
		s := filepath.Join(logDir, "kmsg"+util.RandNameSuffix(t.Name())+".log")
		t.Logf("writing kmsg data to: %s", s)
		f, err := os.Create(s)
		if err != nil {
			t.Errorf("failed to open kmsg file for writing: %v", err)
			return
		}
		defer f.Close()

		ps := testoci.CreateLinuxSpec(ctx, t, vm.ID(),
			testoci.DefaultLinuxSpecOpts(ns.Id,
				ctrdoci.WithDefaultPathEnv,
				ctrdoci.WithProcessArgs("/bin/sh", "-c", `tail +1 -f /dev/kmsg`),
			)...,
		).Process
		p := testcmd.Create(ctx, t, vm, ps, nil)
		p.Stderr = f
		p.Stdout = f

		testcmd.Start(ctx, t, p)

		err = p.Wait()
		t.Logf("[%s] get kmsg events ended: %v", time.Now().Format(time.RFC3339Nano), err)
	}()

	if err := vm.CreateAndAssignNetworkSetup(ctx, "", ""); err != nil {
		t.Fatalf("setting up network: %v", err)
	}
	if err := vm.ConfigureNetworking(ctx, ns.Id); err != nil {
		t.Fatalf("adding network to vm: %v", err)
	}

	// remove the NIC before looping
	if err = vm.RemoveEndpointFromNS(ctx, ns.Id, hnsEP); err != nil {
		t.Fatalf("initial endpoint remove failed: %v", err)
	}

	// start hot adding and removing the endpoint
loop:
	for i := range 2048 {
		if i%50 == 0 {
			t.Logf("iteration #%d", i)
		}

		if err = vm.AddEndpointToNSWithID(ctx, ns.Id, "", hnsEP); err != nil {
			t.Errorf("endpoint add #%d failed at %s: %v", i, time.Now().Format(time.RFC3339Nano), err)
		} else {
			// time.Sleep(5 * time.Millisecond)

			if err = vm.RemoveEndpointFromNS(ctx, ns.Id, hnsEP); err != nil {
				t.Errorf("endpoint remove #%d failed at %s: %v", i, time.Now().Format(time.RFC3339Nano), err)
			}
		}

		if t.Failed() {
			break loop
		}
		time.Sleep(5 * time.Millisecond)
	}

	func() {
		// keep running until the vm is removed
		ctx := context.WithoutCancel(ctx)
		s := filepath.Join(logDir, "dmesg"+util.RandNameSuffix(t.Name())+".log")
		t.Logf("writing dmesg data to: %s", s)
		f, err := os.Create(s)
		if err != nil {
			t.Errorf("failed to open dmesg file for writing: %v", err)
			return
		}
		defer f.Close()

		ps := testoci.CreateLinuxSpec(ctx, t, vm.ID(),
			testoci.DefaultLinuxSpecOpts(ns.Id,
				ctrdoci.WithDefaultPathEnv,
				ctrdoci.WithProcessArgs("/bin/sh", "-c", `dmesg -r`),
			)...,
		).Process
		p := testcmd.Create(ctx, t, vm, ps, nil)
		p.Stderr = f
		p.Stdout = f

		testcmd.Start(ctx, t, p)

		err = p.Wait()
		t.Logf("[%s] get dmesg events ended: %v", time.Now().Format(time.RFC3339Nano), err)
	}()

	// Wait a bit for the trace file to flush
	time.Sleep(1 * time.Second)
	t.Logf("[%s] slept after failure", time.Now().Format(time.RFC3339Nano))
}

func TestLCOW_IPv6_Assignment(t *testing.T) {
	requireFeatures(t, featureLCOW, featureUVM)
	require.Build(t, osversion.RS5)

	ns, err := newNetworkNamespace()
	if err != nil {
		t.Fatalf("namespace creation: %v", err)
	}
	t.Cleanup(func() {
		if err := ns.Delete(); err != nil {
			t.Errorf("namespace delete: %v", err)
		}
	})
	t.Logf("created namespace %s", ns.Id)

	ipv4Route := hcn.Route{
		NextHop:           "192.168.128.1",
		DestinationPrefix: "0.0.0.0/0",
	}

	ipv6Route := hcn.Route{
		NextHop:           "fd00::101",
		DestinationPrefix: "::/0",
	}

	// create network and endpoint
	ntwk, err := (&hcn.HostComputeNetwork{
		Name: hcsOwner + "network",
		Type: hcn.NAT,
		Ipams: []hcn.Ipam{
			{
				Type: "Static",
				Subnets: []hcn.Subnet{
					{
						IpAddressPrefix: "192.168.128.0/20",
						Routes:          []hcn.Route{ipv4Route},
					},
					{
						IpAddressPrefix: "fd00::100/120",
						Routes:          []hcn.Route{ipv6Route},
					},
				},
			},
		},
		SchemaVersion: hcn.Version{Major: 2, Minor: 2},
	}).Create()
	if err != nil {
		t.Fatalf("network creation: %v", err)
	}
	t.Cleanup(func() {
		if err := ntwk.Delete(); err != nil {
			t.Errorf("network delete: %v", err)
		}
	})
	t.Logf("created network %s (%s)", ntwk.Name, ntwk.Id)

	ip4Want := hcn.IpConfig{
		IpAddress:    "192.168.128.4",
		PrefixLength: 20,
	}
	ip6Want := hcn.IpConfig{
		IpAddress:    "fd00::106",
		PrefixLength: 120,
	}

	ep, err := (&hcn.HostComputeEndpoint{
		Name:               ntwk.Name + "endpoint",
		HostComputeNetwork: ntwk.Id,
		Routes:             []hcn.Route{ipv4Route, ipv6Route},
		IpConfigurations:   []hcn.IpConfig{ip4Want, ip6Want},
		SchemaVersion:      hcn.Version{Major: 2, Minor: 2},
	}).Create()
	if err != nil {
		t.Fatalf("endpoint creation: %v", err)
	}
	t.Cleanup(func() {
		if err := ep.Delete(); err != nil {
			t.Errorf("endpoint delete: %v", err)
		}
	})
	t.Logf("created endpoint %s", ep.Id)

	for _, ip := range ep.IpConfigurations {
		if ip != ip4Want && ip != ip6Want {
			t.Fatalf("endpoint address (%v) != %v or %v", ip, ip4Want, ip6Want)
		}
		t.Logf("ip %s/%d", ip.IpAddress, ip.PrefixLength)
	}

	if err := ep.NamespaceAttach(ns.Id); err != nil {
		t.Fatalf("network attachment: %v", err)
	}

	ctx := util.Context(namespacedContext(context.Background()), t)
	ls := linuxImageLayers(ctx, t)
	opts := defaultLCOWOptions(ctx, t)
	vm := testuvm.CreateAndStartLCOWFromOpts(ctx, t, opts)

	if err := vm.CreateAndAssignNetworkSetup(ctx, "", ""); err != nil {
		t.Fatalf("setting up network: %v", err)
	}
	if err := vm.ConfigureNetworking(ctx, ns.Id); err != nil {
		t.Fatalf("adding network to vm: %v", err)
	}

	cID := strings.ReplaceAll(t.Name(), "/", "")
	scratch, _ := testlayers.ScratchSpace(ctx, t, vm, "", "", "")
	spec := testoci.CreateLinuxSpec(ctx, t, cID,
		testoci.DefaultLinuxSpecOpts(ns.Id,
			ctrdoci.WithProcessArgs("/bin/sh", "-c", testoci.TailNullArgs),
			ctrdoci.WithWindowsNetworkNamespace(ns.Id),
			testoci.WithWindowsLayerFolders(append(ls, scratch)))...)

	c, _, cleanup := testcontainer.Create(ctx, t, vm, spec, cID, hcsOwner)
	t.Logf("created container %s", cID)
	t.Cleanup(cleanup)
	init := testcontainer.Start(ctx, t, c, nil)
	t.Cleanup(func() {
		testcmd.Kill(ctx, t, init)
		testcmd.Wait(ctx, t, init)
		testcontainer.Kill(ctx, t, c)
		testcontainer.Wait(ctx, t, c)
	})

	ps := testoci.CreateLinuxSpec(ctx, t, cID,
		testoci.DefaultLinuxSpecOpts(ns.Id,
			ctrdoci.WithDefaultPathEnv,
			ctrdoci.WithProcessArgs("/bin/sh", "-c", `ip -o address show dev eth0 scope global`),
		)...,
	).Process
	io := testcmd.NewBufferedIO()
	p := testcmd.Create(ctx, t, c, ps, io)
	testcmd.Start(ctx, t, p)

	e := testcmd.Wait(ctx, t, p)
	out, err := io.Output()
	t.Logf("cmd output:\n%s", out)
	if e != 0 || err != nil {
		t.Fatalf("exit code %d and error %v", e, err)
	}

	for _, ipc := range ep.IpConfigurations {
		ip := fmt.Sprintf("%s/%d", ipc.IpAddress, ipc.PrefixLength)
		if !strings.Contains(out, ip) {
			t.Errorf("missing ip address %s", ip)
		}
	}
}

func newNetworkNamespace() (*hcn.HostComputeNamespace, error) {
	return (&hcn.HostComputeNamespace{}).Create()
}
