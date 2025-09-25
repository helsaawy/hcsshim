//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/console"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"

	"github.com/Microsoft/hcsshim/internal/cmd"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/uvm"
)

const (
	confidentialArgName  = "confidential"
	vmgsFilePathArgName  = "vmgs-path"
	disableSBArgName     = "disable-secure-boot"
	isolationTypeArgName = "isolation-type"
	writableEFIArgName   = "writable-efi"

	// default policy (that allows all operations) used when no policy is provided
	allowAllPolicy = "cGFja2FnZSBwb2xpY3kKCmFwaV92ZXJzaW9uIDo9ICIwLjExLjAiCmZyYW1ld29ya192ZXJzaW9uIDo9ICIwLjQuMCIKCm1vdW50X2NpbXMgOj0geyJhbGxvd2VkIjogdHJ1ZX0KbW91bnRfZGV2aWNlIDo9IHsiYWxsb3dlZCI6IHRydWV9Cm1vdW50X292ZXJsYXkgOj0geyJhbGxvd2VkIjogdHJ1ZX0KY3JlYXRlX2NvbnRhaW5lciA6PSB7ImFsbG93ZWQiOiB0cnVlLCAiZW52X2xpc3QiOiBudWxsLCAiYWxsb3dfc3RkaW9fYWNjZXNzIjogdHJ1ZX0KdW5tb3VudF9kZXZpY2UgOj0geyJhbGxvd2VkIjogdHJ1ZX0KdW5tb3VudF9vdmVybGF5IDo9IHsiYWxsb3dlZCI6IHRydWV9CmV4ZWNfaW5fY29udGFpbmVyIDo9IHsiYWxsb3dlZCI6IHRydWUsICJlbnZfbGlzdCI6IG51bGx9CmV4ZWNfZXh0ZXJuYWwgOj0geyJhbGxvd2VkIjogdHJ1ZSwgImVudl9saXN0IjogbnVsbCwgImFsbG93X3N0ZGlvX2FjY2VzcyI6IHRydWV9CnNodXRkb3duX2NvbnRhaW5lciA6PSB7ImFsbG93ZWQiOiB0cnVlfQpzaWduYWxfY29udGFpbmVyX3Byb2Nlc3MgOj0geyJhbGxvd2VkIjogdHJ1ZX0KcGxhbjlfbW91bnQgOj0geyJhbGxvd2VkIjogdHJ1ZX0KcGxhbjlfdW5tb3VudCA6PSB7ImFsbG93ZWQiOiB0cnVlfQpnZXRfcHJvcGVydGllcyA6PSB7ImFsbG93ZWQiOiB0cnVlfQpkdW1wX3N0YWNrcyA6PSB7ImFsbG93ZWQiOiB0cnVlfQpydW50aW1lX2xvZ2dpbmcgOj0geyJhbGxvd2VkIjogdHJ1ZX0KbG9hZF9mcmFnbWVudCA6PSB7ImFsbG93ZWQiOiB0cnVlfQpzY3JhdGNoX21vdW50IDo9IHsiYWxsb3dlZCI6IHRydWV9CnNjcmF0Y2hfdW5tb3VudCA6PSB7ImFsbG93ZWQiOiB0cnVlfQo="
)

var (
	cwcowBootVHD           string
	cwcowEFIVHD            string
	cwcowScratchVHD        string
	cwcowVMGSPath          string
	cwcowDisableSecureBoot bool
	cwcowIsolationMode     string
	cwcowSecurityPolicy    string
	cwcowWritableEFI       bool
)

var cwcowCommand = cli.Command{
	Name:  "cwcow",
	Usage: "boot a confidential WCOW UVM",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:        "efi-vhd",
			Usage:       "`VHD` at the provided path MUST have the EFI boot partition and be properly formatted for UEFI boot.",
			Destination: &cwcowEFIVHD,
			Required:    true,
		},
		cli.StringFlag{
			Name:        "boot-cim-vhd",
			Usage:       "A `VHD` containing the block CIM that contains the OS files.",
			Destination: &cwcowBootVHD,
			Required:    true,
		},
		cli.StringFlag{
			Name:        "scratch-vhd",
			Usage:       "A scratch `VHD` for the UVM",
			Destination: &cwcowScratchVHD,
			Required:    true,
		},
		cli.StringFlag{
			Name:        vmgsFilePathArgName,
			Usage:       "`VMGS` file path (only applies when confidential mode is enabled). This option is only applicable in confidential mode.",
			Destination: &cwcowVMGSPath,
			Required:    true,
		},
		cli.BoolFlag{
			Name:        disableSBArgName,
			Usage:       "Disables Secure Boot when running the UVM in confidential mode. This option is only applicable in confidential mode.",
			Destination: &cwcowDisableSecureBoot,
		},
		cli.StringFlag{
			Name:        isolationTypeArgName,
			Usage:       "VM Isolation type (one of Disabled, GuestStateOnly, VirtualizationBasedSecurity, SecureNestedPaging or TrustDomain). Applicable only when using the confidential mode. This option is only applicable in confidential mode.",
			Destination: &cwcowIsolationMode,
			Required:    true,
		},
		cli.StringFlag{
			Name:        securityPolicyArgName,
			Usage:       "Security `policy` that should be enforced inside the UVM. If none is provided, default policy that allows all operations will be used.",
			Destination: &cwcowSecurityPolicy,
			Value:       allowAllPolicy,
		},
		cli.BoolFlag{
			Name:        writableEFIArgName,
			Usage:       "Attaches the EFI `VHD` as read-write instead of read-only. This allows the UVM to modify the contents of the VHD, be careful when using this option!",
			Destination: &cwcowWritableEFI,
		},
	},
	Action: func(cCtx *cli.Context) error {
		runMany(cCtx, func(id string) error {
			ctx := context.Background()

			options, err := uVMCreateOptionsCommon[*uvm.OptionsWCOW](ctx, cCtx, id, "")
			if err != nil {
				return err
			}
			if options.ProcessorCount == 0 { // not overridden by flag
				options.ProcessorCount = 2
			}
			if options.MemorySizeInMB == 0 { // not overridden by flag
				options.MemorySizeInMB = 2048
			}
			options.AllowOvercommit = false
			options.EnableDeferredCommit = false

			// confidential specific options
			options.SecurityPolicyEnabled = true
			options.SecurityPolicy = cwcowSecurityPolicy
			options.DisableSecureBoot = cwcowDisableSecureBoot
			options.GuestStateFilePath = cwcowVMGSPath
			options.IsolationType = cwcowIsolationMode
			// always enable graphics console with uvmboot - helps with testing/debugging
			options.EnableGraphicsConsole = true
			options.WritableEFI = cwcowWritableEFI

			cwcowBootVHD, err = filepath.Abs(cwcowBootVHD)
			if err != nil {
				return err
			}

			cwcowEFIVHD, err = filepath.Abs(cwcowEFIVHD)
			if err != nil {
				return err
			}

			cwcowScratchVHD, err = filepath.Abs(cwcowScratchVHD)
			if err != nil {
				return err
			}

			options.BootFiles = &uvm.WCOWBootFiles{
				BootType: uvm.BlockCIMBoot,
				BlockCIMFiles: &uvm.BlockCIMBootFiles{
					BootCIMVHDPath: cwcowBootVHD,
					EFIVHDPath:     cwcowEFIVHD,
					ScratchVHDPath: cwcowScratchVHD,
				},
			}

			vm, err := uvm.CreateWCOW(ctx, options)
			if err != nil {
				return fmt.Errorf("create uVM: %w", err)
			}
			defer vm.Close()
			if err := vm.Start(ctx); err != nil {
				return fmt.Errorf("start uVM: %w", err)
			}

			if commandLine := cCtx.String(execCommandLineArgName); commandLine != "" {
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
						csz, err := con.Size()
						if err != nil {
							return fmt.Errorf("failed to get console size: %w", err)
						}
						c.Spec.ConsoleSize = &specs.Box{
							Height: uint(csz.Height),
							Width:  uint(csz.Width),
						}
						err = con.SetRaw()
						if err != nil {
							return fmt.Errorf("failed to set console to raw mode: %w", err)
						}
						defer func() {
							_ = con.Reset()
						}()
					}
				} else {
					c.Stdout = os.Stdout
					c.Stderr = os.Stdout
				}
				err = c.Run()
				if err != nil {
					return err
				}
			}
			_ = vm.Terminate(ctx)
			_ = vm.Wait()
			return vm.ExitError()
		})
		return nil
	},
}
