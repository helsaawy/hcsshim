//go:build windows && functional
// +build windows,functional

package cri_containerd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Microsoft/hcsshim/pkg/annotations"
)

func TestLegacyDrivers(t *testing.T) {
	requireFeatures(t, featureWCOWHypervisor, featureLegacyDrivers)

	_, err := os.Stat(*flagLegacyDriversDir)
	if err != nil {
		t.Skipf("could access legacy drivers directory %q: %v", *flagLegacyDriversDir, err)
	}

	fs, err := os.ReadDir(*flagLegacyDriversDir)
	must(t, err, "could not enumerate legacy drivers in directory "+*flagLegacyDriversDir)

	drivers := make([]annotations.Driver, len(fs))
	for i, f := range fs {
		drivers[i] = annotations.Driver{
			Path: filepath.Join(*flagLegacyDriversDir, f.Name()),
			Type: annotations.DriverTypeWindowsLegacy,
		}
	}
	driversPayload, err := annotations.CreateDriverAnnotationPayload(drivers...)
	must(t, err, "could not driver payload")
	t.Logf("driver annotation payload: %s", driversPayload)

	client := newTestRuntimeClient(t)
	ctx := context.Background()

	tests := []struct {
		runtime string
		image   string
	}{
		{
			runtime: wcowHypervisor17763RuntimeHandler,
			image:   imageWindowsNanoserver17763,
		},
		{
			runtime: wcowHypervisor18362RuntimeHandler,
			image:   imageWindowsNanoserver18362,
		},
		{
			runtime: wcowHypervisor19041RuntimeHandler,
			image:   imageWindowsNanoserver19041,
		},
		{
			runtime: wcowHypervisorRuntimeHandler,
			image:   imageWindowsNanoserver,
		},
	}

	for _, tc := range tests {
		t.Run(tc.runtime, func(t *testing.T) {
			pullRequiredImages(t, []string{tc.image})

			podRequest := getRunPodSandboxRequest(t, tc.runtime)
			podID := runPodSandbox(t, client, ctx, podRequest)
			defer removePodSandbox(t, client, ctx, podID)
			defer stopPodSandbox(t, client, ctx, podID)

			ctrRequest := getCreateContainerRequest(podID, tc.runtime+"-driver-test", tc.image,
				[]string{"cmd", "/c", "ping -t 127.0.0.1"}, podRequest.Config)
			ctrRequest.Config.Annotations = map[string]string{
				annotations.VirtualMachineKernelDrivers: driversPayload,
			}
			ctrID := createContainer(t, client, ctx, ctrRequest)
			startContainer(t, client, ctx, ctrID)
			defer removeContainer(t, client, ctx, ctrID)
			defer stopContainer(t, client, ctx, ctrID)
		})
	}
}

func must(tb testing.TB, err error, msgs ...string) {
	if err == nil {
		return
	}
	tb.Helper()
	tb.Fatalf(msgJoin(msgs, "%v"), err)
}

func msgJoin(pre []string, s string) string {
	return strings.Join(append(pre, s), ": ")
}
