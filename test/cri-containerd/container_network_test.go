//go:build windows && functional
// +build windows,functional

package cri_containerd

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	runtime "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

func Test_Container_Network_LCOW(t *testing.T) {
	requireFeatures(t, featureLCOW)

	pullRequiredLCOWImages(t, []string{imageLcowK8sPause, imageLcowAlpine})

	// create a directory and log file
	dir := t.TempDir()
	log := filepath.Join(dir, "ping.txt")

	sandboxRequest := getRunPodSandboxRequest(t, lcowRuntimeHandler)

	client := newTestRuntimeClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	podID := runPodSandbox(t, client, ctx, sandboxRequest)
	defer removePodSandbox(t, client, ctx, podID)
	defer stopPodSandbox(t, client, ctx, podID)

	request := &runtime.CreateContainerRequest{
		PodSandboxId: podID,
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageLcowAlpine,
			},
			Command: []string{
				"ping",
				"-q", // -q outputs ping stats only.
				"-c",
				"10",
				"google.com",
			},
			LogPath: log,
			Linux:   &runtime.LinuxContainerConfig{},
		},
		SandboxConfig: sandboxRequest.Config,
	}

	containerID := createContainer(t, client, ctx, request)
	defer removeContainer(t, client, ctx, containerID)

	startContainer(t, client, ctx, containerID)
	defer stopContainer(t, client, ctx, containerID)

	// wait a while for container to write to stdout
	time.Sleep(3 * time.Second)

	// open the log and test for any packet loss
	logFile, err := os.Open(log)
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()

	s := bufio.NewScanner(logFile)
	for s.Scan() {
		v := strings.Fields(s.Text())
		t.Logf("ping output: %v", v)

		if v != nil && v[len(v)-1] == "loss" && v[len(v)-3] != "0%" {
			t.Fatalf("expected 0%% packet loss, got %v packet loss", v[len(v)-3])
		}
	}
}

func Test_Container_Network_Hostname(t *testing.T) {
	type config struct {
		name             string
		requiredFeatures []string
		runtimeHandler   string
		sandboxImage     string
		containerImage   string
		cmd              []string
	}
	tests := []config{
		{
			name:             "WCOW_Process",
			requiredFeatures: []string{featureWCOWProcess},
			runtimeHandler:   wcowProcessRuntimeHandler,
			sandboxImage:     imageWindowsNanoserver,
			containerImage:   imageWindowsNanoserver,
			cmd:              []string{"cmd", "/c", "ping", "-t", "127.0.0.1"},
		},
		{
			name:             "WCOW_Hypervisor",
			requiredFeatures: []string{featureWCOWHypervisor},
			runtimeHandler:   wcowHypervisorRuntimeHandler,
			sandboxImage:     imageWindowsNanoserver,
			containerImage:   imageWindowsNanoserver,
			cmd:              []string{"cmd", "/c", "ping", "-t", "127.0.0.1"},
		},
		{
			name:             "LCOW",
			requiredFeatures: []string{featureLCOW},
			runtimeHandler:   lcowRuntimeHandler,
			sandboxImage:     imageLcowK8sPause,
			containerImage:   imageLcowAlpine,
			cmd:              []string{"top"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requireFeatures(t, test.requiredFeatures...)

			if test.runtimeHandler == lcowRuntimeHandler {
				pullRequiredLCOWImages(t, []string{test.sandboxImage, test.containerImage})
			} else {
				pullRequiredImages(t, []string{test.sandboxImage, test.containerImage})
			}

			sandboxRequest := getRunPodSandboxRequest(t, test.runtimeHandler)
			sandboxRequest.Config.Hostname = "TestHost"

			client := newTestRuntimeClient(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			podID := runPodSandbox(t, client, ctx, sandboxRequest)
			defer removePodSandbox(t, client, ctx, podID)
			defer stopPodSandbox(t, client, ctx, podID)

			containerRequest := &runtime.CreateContainerRequest{
				PodSandboxId: podID,
				Config: &runtime.ContainerConfig{
					Metadata: &runtime.ContainerMetadata{
						Name: t.Name() + "-Container",
					},
					Image: &runtime.ImageSpec{
						Image: test.containerImage,
					},
					Command: test.cmd,
				},
				SandboxConfig: sandboxRequest.Config,
			}

			containerID := createContainer(t, client, ctx, containerRequest)
			defer removeContainer(t, client, ctx, containerID)

			startContainer(t, client, ctx, containerID)
			defer stopContainer(t, client, ctx, containerID)

			execResponse := execSync(t, client, ctx, &runtime.ExecSyncRequest{
				ContainerId: containerID,
				Cmd:         []string{"hostname"},
			})
			stdout := strings.Trim(string(execResponse.Stdout), " \r\n")
			if stdout != sandboxRequest.Config.Hostname {
				t.Fatalf("expected hostname: '%s', got '%s'", sandboxRequest.Config.Hostname, stdout)
			}
		})
	}
}

func Test_Container_Network_PingHost(t *testing.T) {
	client := newTestRuntimeClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tests := []struct {
		name    string
		feature string
		runtime string
		image   string
		command []string
		ping    []string
	}{
		{
			name:    "LCOW",
			feature: featureLCOW,
			runtime: lcowRuntimeHandler,
			image:   imageLcowAlpine,
			command: []string{
				"ash",
				"-c",
				"tail -f /dev/null",
			},
			ping: []string{"ping", "-q", "-c", "4"},
		},
		{
			name:    "WCOW_Hypervisor",
			feature: featureWCOWHypervisor,
			runtime: wcowHypervisorRuntimeHandler,
			image:   imageWindowsNanoserver,
			command: []string{
				"cmd",
				"/c",
				"ping -t 127.0.0.1",
			},
			ping: []string{"cmd", "/c", "ping -n 4"},
		},
		{
			name:    "WCOW_Process",
			feature: featureWCOWProcess,
			runtime: wcowProcessRuntimeHandler,
			image:   imageWindowsNanoserver,
			command: []string{
				"cmd",
				"/c",
				"ping -t 127.0.0.1",
			},
			ping: []string{"cmd", "/c", "ping -n 4"},
		},
	}

	b, err := exec.CommandContext(
		ctx,
		"powershell.exe",
		"/c",
		`(Get-NetIPAddress -AddressFamily IPv4 -InterfaceAlias "*ContainerPlat*").IPAddress`,
	).Output()
	if err != nil {
		t.Fatalf("could not retrieve ContainerPlat IP address: %v", err)
	}
	hostIP := strings.TrimSpace(string(b))
	t.Log("host IP:", hostIP)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireFeatures(t, tt.feature)

			switch tt.feature {
			case featureLCOW:
				pullRequiredLCOWImages(t, append([]string{imageLcowK8sPause}, tt.image))
			case featureWCOWHypervisor, featureWCOWProcess:
				pullRequiredImages(t, []string{tt.image})
			}

			sandboxRequest := getRunPodSandboxRequest(t, tt.runtime)
			podID := runPodSandbox(t, client, ctx, sandboxRequest)
			defer removePodSandbox(t, client, ctx, podID)
			defer stopPodSandbox(t, client, ctx, podID)

			request := getCreateContainerRequest(podID, t.Name()+"-Container", tt.image, tt.command, sandboxRequest.Config)
			containerID := createContainer(t, client, ctx, request)
			startContainer(t, client, ctx, containerID)
			defer removeContainer(t, client, ctx, containerID)
			defer stopContainer(t, client, ctx, containerID)

			pingCmd := append(tt.ping, hostIP)
			req := execSync(t, client, ctx, &runtime.ExecSyncRequest{
				ContainerId: containerID,
				Cmd:         pingCmd,
				Timeout:     30,
			})
			if req.ExitCode != 0 {
				t.Fatalf("exec %v failed with exit code %d: %s", pingCmd, req.ExitCode, string(req.Stderr))
			}
			t.Logf("exec: %s, %s", pingCmd, req.Stdout)
		})
	}
}

func Test_Container_Network_PingContainer(t *testing.T) {
	client := newTestRuntimeClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tests := []struct {
		name    string
		feature string
		runtime string
		image   string
		command []string
		port    int32
		ping    []string
	}{
		{
			name:    "LCOW",
			feature: featureLCOW,
			runtime: lcowRuntimeHandler,
			image:   imageLcowAlpine,
			command: []string{
				"ash",
				"-c",
				"tail -f /dev/null",
			},
			port: 445,
			ping: []string{"ping", "-q", "-c", "4"},
		},
		{
			name:    "WCOW_Hypervisor",
			feature: featureWCOWHypervisor,
			runtime: wcowHypervisorRuntimeHandler,
			image:   imageWindowsNanoserver,
			command: []string{
				"cmd",
				"/c",
				"ping -t 127.0.0.1",
			},
			port: 445,
			ping: []string{"cmd", "/c", "ping -n 4"},
		},
		{
			name:    "WCOW_Process",
			feature: featureWCOWProcess,
			runtime: wcowProcessRuntimeHandler,
			image:   imageWindowsNanoserver,
			command: []string{
				"cmd",
				"/c",
				"ping -t 127.0.0.1",
			},
			port: 445,
			ping: []string{"cmd", "/c", "ping -n 4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireFeatures(t, tt.feature)

			switch tt.feature {
			case featureLCOW:
				pullRequiredLCOWImages(t, append([]string{imageLcowK8sPause}, tt.image))
			case featureWCOWHypervisor, featureWCOWProcess:
				pullRequiredImages(t, []string{tt.image})
			}

			sandboxRequest := getRunPodSandboxRequest(t, tt.runtime, WithPortMapping(runtime.Protocol_TCP, tt.port, 8446, ""))
			podID := runPodSandbox(t, client, ctx, sandboxRequest)
			defer removePodSandbox(t, client, ctx, podID)
			defer stopPodSandbox(t, client, ctx, podID)
			status := getPodSandboxStatus(t, client, ctx, podID)
			podIP := status.Network.Ip
			t.Log("pod IP:", podIP)

			request := getCreateContainerRequest(podID, t.Name()+"-Container", tt.image, tt.command, sandboxRequest.Config)
			containerID := createContainer(t, client, ctx, request)
			startContainer(t, client, ctx, containerID)
			defer removeContainer(t, client, ctx, containerID)
			defer stopContainer(t, client, ctx, containerID)

			pingCmd := []string{"/c", "exit -not (Test-NetConnection -InformationLevel Quiet -p 8446 " + podIP + ")"}
			t.Logf("ping command: powershell %s", pingCmd)
			b, err := exec.CommandContext(ctx, "powershell", pingCmd...).CombinedOutput()
			if err != nil {
				t.Fatalf("could not ping container: %v\n%s", err, b)
			}
			t.Logf("%s", b)
		})
	}
}

func Test_Container_Network_WebServer_PortForward(t *testing.T) {
	client := newTestRuntimeClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cPort := 80
	hostPort := 8080

	tests := []struct {
		name    string
		feature string
		runtime string
		image   string
	}{
		{
			name:    "LCOW",
			feature: featureLCOW,
			runtime: lcowRuntimeHandler,
			image:   imageLinuxPython,
		},
		{
			name:    "WCOW_Hypervisor",
			feature: featureWCOWHypervisor,
			runtime: wcowHypervisorRuntimeHandler,
			image:   imageWindowsPython,
		},
		{
			name:    "WCOW_Process",
			feature: featureWCOWProcess,
			runtime: wcowProcessRuntimeHandler,
			image:   imageWindowsPython,
		},
	}
	// TODO:
	// curl self from inside container
	// curl from host
}
