//go:build windows

package main

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/containerd/console"
	"github.com/urfave/cli"

	"github.com/Microsoft/hcsshim/internal/cmd"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/memory"
	"github.com/Microsoft/hcsshim/internal/uvm"
)

const (
	bootFilesPathArgName          = "boot-files-path"
	consolePipeArgName            = "console-pipe"
	kernelDirectArgName           = "kernel-direct"
	kernelFileArgName             = "kernel-file"
	kernelArgsArgName             = "kernel-args"
	rootFSTypeArgName             = "root-fs-type"
	disableTimeSyncArgName        = "disable-time-sync"
	vpMemMaxCountArgName          = "vpmem-max-count"
	vpMemMaxSizeArgName           = "vpmem-max-size"
	scsiMountsArgName             = "mount-scsi"
	vpmemMountsArgName            = "mount-vpmem"
	shareFilesArgName             = "share"
	securityPolicyArgName         = "security-policy"
	securityHardwareFlag          = "security-hardware"
	securityPolicyEnforcerArgName = "security-policy-enforcer"
)

var lcowCommand = cli.Command{
	Name:  "lcow",
	Usage: "Boot an LCOW UVM",
	CustomHelpTemplate: cli.CommandHelpTemplate + "EXAMPLES:\n" +
		`   .\uvmboot.exe -gcs lcow -boot-files-path "C:\ContainerPlat\LinuxBootFiles" -root-fs-type vhd -t -exec "/bin/bash"`,
	Flags: append(commonUVMFlags,
		cli.StringFlag{
			Name:  kernelArgsArgName,
			Value: "",
			Usage: "Additional arguments to pass to the kernel",
		},
		cli.StringFlag{
			Name:  rootFSTypeArgName,
			Usage: "Either 'initrd', 'vhd' or 'none'. (default: 'vhd' if rootfs.vhd exists)",
		},
		cli.StringFlag{
			Name:  bootFilesPathArgName,
			Usage: "The `path` to the boot files directory",
		},
		cli.UintFlag{
			Name:  vpMemMaxCountArgName,
			Usage: "Number of VPMem devices on the UVM. Uses hcsshim default if not specified",
		},
		cli.Uint64Flag{
			Name:  vpMemMaxSizeArgName,
			Usage: "Size of each VPMem device, in MB. Uses hcsshim default if not specified",
		},
		cli.BoolFlag{
			Name:  kernelDirectArgName,
			Usage: "Use kernel direct booting for UVM (default: true on builds >= 18286)",
		},
		cli.StringFlag{
			Name:  kernelFileArgName,
			Usage: "The kernel `file` to use; either 'kernel' or 'vmlinux'. (default: 'kernel')",
		},
		cli.BoolFlag{
			Name:  disableTimeSyncArgName,
			Usage: "Disable the time synchronization service",
		},
		cli.StringFlag{
			Name:  securityPolicyArgName,
			Usage: "Security policy to set on the UVM. Leave empty to use an open door policy",
		},
		cli.StringFlag{
			Name: securityPolicyEnforcerArgName,
			Usage: "Security policy enforcer to use for a given security policy. " +
				"Leave empty to use the default enforcer",
		},
		cli.BoolFlag{
			Name:  securityHardwareFlag,
			Usage: "Use VMGS file to run on secure hardware. ('root-fs-type' must be set to 'none')",
		},
		cli.StringFlag{
			Name:  consolePipeArgName,
			Usage: "Named pipe for serial console output (which will be enabled)",
		},
		cli.StringSliceFlag{
			Name: scsiMountsArgName,
			Usage: "List of VHDs to SCSI mount into the UVM. Use repeat instances to add multiple. " +
				"Value is of the form `'host[,guest[,w]]'`, where 'host' is path to the VHD, " +
				`'guest' is an optional mount path inside the UVM, and 'w' mounts the VHD as writeable`,
		},
		cli.StringSliceFlag{
			Name: shareFilesArgName,
			Usage: "List of paths or files to plan9 share into the UVM. Use repeat instances to add multiple. " +
				"Value is of the form `'host,guest[,w]' where 'host' is path to the VHD, " +
				`'guest' is the mount path inside the UVM, and 'w' sets the shared files to writeable`,
		},
		cli.StringSliceFlag{
			Name:  vpmemMountsArgName,
			Usage: "List of VHDs to VPMem mount into the UVM. Use repeat instances to add multiple. ",
		},
	),
	Action: func(cCtx *cli.Context) error {
		runMany(cCtx, func(id string) error {
			ctx := context.Background()

			options, err := createLCOWOptions(ctx, cCtx, id)
			if err != nil {
				return err
			}

			return runLCOW(ctx, cCtx, options)
		})

		return nil
	},
}

func createLCOWOptions(ctx context.Context, cCtx *cli.Context, id string) (*uvm.OptionsLCOW, error) {
	options := uvm.NewDefaultOptionsLCOW(id, "")
	setGlobalOptions(cCtx, options.Options)

	// boot
	if cCtx.IsSet(bootFilesPathArgName) {
		options.UpdateBootFilesPath(ctx, cCtx.String(bootFilesPathArgName))
	}

	// kernel
	if cCtx.IsSet(kernelDirectArgName) {
		options.KernelDirect = cCtx.Bool(kernelDirectArgName)
	}
	if cCtx.IsSet(kernelFileArgName) {
		switch strings.ToLower(cCtx.String(kernelFileArgName)) {
		case uvm.KernelFile:
			options.KernelFile = uvm.KernelFile
		case uvm.UncompressedKernelFile:
			options.KernelFile = uvm.UncompressedKernelFile
		default:
			return nil, unrecognizedError(cCtx.String(kernelFileArgName), kernelFileArgName)
		}
	}
	if cCtx.IsSet(kernelArgsArgName) {
		options.KernelBootOptions = cCtx.String(kernelArgsArgName)
	}

	// rootfs
	if cCtx.IsSet(rootFSTypeArgName) {
		switch strings.ToLower(cCtx.String(rootFSTypeArgName)) {
		case "initrd":
			options.RootFSFile = uvm.InitrdFile
			options.PreferredRootFSType = uvm.PreferredRootFSTypeInitRd
		case "vhd":
			options.RootFSFile = uvm.VhdFile
			options.PreferredRootFSType = uvm.PreferredRootFSTypeVHD
		case "none":
			options.RootFSFile = ""
			options.PreferredRootFSType = uvm.PreferredRootFSTypeNA
		default:
			return nil, unrecognizedError(cCtx.String(rootFSTypeArgName), rootFSTypeArgName)
		}
	}

	if cCtx.IsSet(vpMemMaxCountArgName) {
		options.VPMemDeviceCount = uint32(cCtx.Uint(vpMemMaxCountArgName))
	}
	if cCtx.IsSet(vpMemMaxSizeArgName) {
		options.VPMemSizeBytes = cCtx.Uint64(vpMemMaxSizeArgName) * memory.MiB // convert from MB to bytes
	}

	// GCS
	options.UseGuestConnection = cCtx.GlobalBool(useGCSArgName)
	if !options.UseGuestConnection {
		if cCtx.IsSet(execCommandLineArgName) {
			options.ExecCommandLine = cCtx.String(execCommandLineArgName)
		}
		if cCtx.IsSet(forwardStdoutArgName) {
			options.ForwardStdout = cCtx.Bool(forwardStdoutArgName)
		}
		if cCtx.IsSet(forwardStderrArgName) {
			options.ForwardStderr = cCtx.Bool(forwardStderrArgName)
		}
		if cCtx.IsSet(outputHandlingArgName) {
			switch strings.ToLower(cCtx.String(outputHandlingArgName)) {
			case "stdout":
				options.OutputHandlerCreator = func(*uvm.Options) uvm.OutputHandler {
					return func(r io.Reader) {
						_, _ = io.Copy(os.Stdout, r)
					}
				}
			default:
				return nil, unrecognizedError(cCtx.String(outputHandlingArgName), outputHandlingArgName)
			}
		}
	}
	if cCtx.IsSet(consolePipeArgName) {
		options.ConsolePipe = cCtx.String(consolePipeArgName)
	}

	// general settings
	if cCtx.IsSet(disableTimeSyncArgName) {
		options.DisableTimeSyncService = cCtx.Bool(disableTimeSyncArgName)
	}

	// empty policy string defaults to open door
	if cCtx.IsSet(securityPolicyArgName) {
		options.SecurityPolicy = cCtx.String(securityPolicyArgName)
	}
	if cCtx.IsSet(securityPolicyEnforcerArgName) {
		options.SecurityPolicyEnforcer = cCtx.String(securityPolicyEnforcerArgName)
	}
	if cCtx.IsSet(securityHardwareFlag) {
		options.GuestStateFile = uvm.GuestStateFile
		options.SecurityPolicyEnabled = true
		options.AllowOvercommit = false
	}

	return options, nil
}

func runLCOW(ctx context.Context, cCtx *cli.Context, options *uvm.OptionsLCOW) error {
	vm, err := uvm.CreateLCOW(ctx, options)
	if err != nil {
		return err
	}
	defer func() {
		_ = vm.CloseCtx(ctx)
	}()

	if err := vm.Start(ctx); err != nil {
		return err
	}

	if err := mountSCSI(ctx, cCtx, vm); err != nil {
		return err
	}

	if err := shareFiles(ctx, cCtx, vm); err != nil {
		return err
	}

	if err := mountVPMem(ctx, cCtx, vm); err != nil {
		return err
	}

	if options.UseGuestConnection {
		if err := execViaGCS(ctx, vm, cCtx); err != nil {
			return err
		}
		_ = vm.Terminate(ctx)
		_ = vm.WaitCtx(ctx)

		return vm.ExitError()
	}

	return vm.WaitCtx(ctx)
}

func execViaGCS(ctx context.Context, vm *uvm.UtilityVM, cCtx *cli.Context) error {
	c := cmd.CommandContext(ctx, vm, "sh", "-c", cCtx.String(execCommandLineArgName))
	c.Log = log.L.Dup()
	if cCtx.Bool(useTerminalArgName) {
		c.Spec.Terminal = true
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		con, err := console.ConsoleFromFile(os.Stdin)
		if err != nil {
			log.G(ctx).WithError(err).Warn("could not create console from stdin")
		} else {
			if err := con.SetRaw(); err != nil {
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
			c.Stderr = os.Stdout // match non-GCS behavior and forward to stdout
		}
	}

	return c.Run()
}
