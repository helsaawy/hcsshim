//go:build windows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/console"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/Microsoft/hcsshim/internal/cmd"
	"github.com/Microsoft/hcsshim/internal/layers"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/Microsoft/hcsshim/internal/wclayer"
)

type cleanupFn func(context.Context)

const (
	wcowDockerImageArgName  = "docker-image"
	wcowImagePathArgName    = "image"
	wcowNoCMDPrependArgName = "no-cmd"
)

var wcowCommand = cli.Command{
	Name:  "wcow",
	Usage: "boot a WCOW UVM",
	Flags: append(commonUVMFlags,
		cli.StringFlag{
			Name:  wcowDockerImageArgName,
			Usage: "Docker `image` to use for the UVM image",
		},
		// TODO: make this a StringSliceFlag, and allow passing in an array of layers
		cli.StringFlag{
			Name:  wcowImagePathArgName,
			Usage: "Path for the UVM boot `image`",
		},
		cli.BoolFlag{
			Name:  wcowNoCMDPrependArgName,
			Usage: "Don't prepend 'cmd /c' to the exec command",
		},
	),
	Action: func(cCtx *cli.Context) error {
		runMany(cCtx, func(id string) error {
			ctx := context.Background()

			options, cleanup, err := createWCOWOptions(ctx, cCtx, id)
			defer func() { // schedule the cleanup first, since it may be non-nil regardless of the error
				if cleanup == nil {
					return
				}
				cleanup(ctx)
			}()
			if err != nil {
				return err
			}

			return runWCOW(ctx, cCtx, options)
		})

		return nil
	},
}

func createWCOWOptions(ctx context.Context, cCtx *cli.Context, id string) (*uvm.OptionsWCOW, cleanupFn, error) {
	options := uvm.NewDefaultOptionsWCOW(id, "")
	setGlobalOptions(cCtx, options.Options)

	var layerFolders []string
	if wcowImage := cCtx.String(wcowImagePathArgName); wcowImage != "" {
		layer, err := filepath.Abs(wcowImage)
		if err != nil {
			return nil, nil, err
		}
		layerFolders = []string{layer}
	} else {
		wcowDockerImage := cCtx.String(wcowDockerImageArgName)
		if wcowDockerImage == "" {
			wcowDockerImage = "mcr.microsoft.com/windows/nanoserver:1809"
		}
		var err error
		layerFolders, err = getLayers(wcowDockerImage)
		if err != nil {
			return nil, nil, err
		}
	}

	tempDir, err := os.MkdirTemp("", "uvmboot")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func(ctx context.Context) {
		if err := destroyLayer(ctx, tempDir); err != nil {
			log.G(ctx).WithFields(logrus.Fields{
				logrus.ErrorKey: err,
				"directory":     tempDir,
			}).Warn("could not destroy scratch directory")
		}
	}

	layerFolders = append(layerFolders, tempDir)
	options.BootFiles, err = layers.GetWCOWUVMBootFilesFromLayers(ctx, nil, layerFolders)
	if err != nil {
		cleanup(ctx)
		return nil, nil, err
	}
	return options, cleanup, nil
}

func runWCOW(ctx context.Context, cCtx *cli.Context, options *uvm.OptionsWCOW) error {
	vm, err := uvm.CreateWCOW(ctx, options)
	if err != nil {
		return err
	}
	defer func() {
		_ = vm.CloseCtx(ctx)
	}()

	if err := vm.Start(ctx); err != nil {
		return err
	}

	if commandLine := cCtx.String(execCommandLineArgName); commandLine != "" {
		logrus.WithField("command", commandLine).Debug("creating exec command")
		var c *cmd.Cmd
		if cCtx.Bool(wcowNoCMDPrependArgName) {
			// Cmd on Windows host doesn't use arg array, except when escapping them to create [c.Spec.CommandLine]
			// we can play fast and loose with the arguments themselves if we are providing the CommandLine directly
			c = cmd.CommandContext(ctx, vm, commandLine)
			c.Spec.Args = nil
			c.Spec.CommandLine = commandLine
		} else {
			c = cmd.CommandContext(ctx, vm, "cmd.exe", "/c", commandLine)
		}
		c.Spec.User.Username = `NT AUTHORITY\SYSTEM`
		c.Log = log.L.Dup()
		if cCtx.Bool(useTerminalArgName) {
			c.Spec.Terminal = true
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			con, err := console.ConsoleFromFile(os.Stdin)
			if err == nil {
				err = con.SetRaw()
				if err != nil {
					return err
				}
				defer func() {
					_ = con.Reset()
				}()
			}
		} else if cCtx.String(outputHandlingArgName) == "stdout" {
			if cCtx.Bool(forwardStdoutArgName) {
				c.Stdout = os.Stdout
			}
			if cCtx.Bool(forwardStderrArgName) {
				c.Stderr = os.Stdout
			}
		}
		err = c.Run()
		if err != nil {
			return err
		}
	}

	_ = vm.Terminate(ctx)
	_ = vm.Wait()
	return vm.ExitError()
}

func getLayers(imageName string) ([]string, error) {
	c := exec.Command("docker", "inspect", imageName, "-f", `"{{.GraphDriver.Data.dir}}"`)
	out, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to find layers for %s", imageName)
	}
	imagePath := strings.Replace(strings.TrimSpace(string(out)), `"`, ``, -1)
	layers, err := getLayerChain(imagePath)
	if err != nil {
		return nil, err
	}
	return append([]string{imagePath}, layers...), nil
}

func getLayerChain(layerFolder string) ([]string, error) {
	jPath := filepath.Join(layerFolder, "layerchain.json")
	content, err := os.ReadFile(jPath)
	if err != nil {
		return nil, err
	}
	var layerChain []string
	err = json.Unmarshal(content, &layerChain)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal layerchain: %w", err)
	}
	return layerChain, nil
}

// TODO: move DestroyLayer out of "github.com/Microsoft/hcsshim/test/internal/util" and use it here
func destroyLayer(ctx context.Context, p string) (err error) {
	// check if the path exists
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return nil
	}

	repeat := func(f func() error, n int, d time.Duration) (err error) {
		if n < 1 {
			n = 1
		}

		err = f()
		for i := 1; i < n; i++ {
			if err == nil {
				break
			}

			time.Sleep(d)
			err = f()
		}

		return err
	}

	return repeat(func() error { return wclayer.DestroyLayer(ctx, p) }, 3, time.Millisecond)
}
