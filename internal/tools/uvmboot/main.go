//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"go.opencensus.io/trace"

	"github.com/Microsoft/hcsshim/internal/oc"
	"github.com/Microsoft/hcsshim/internal/oci"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/Microsoft/hcsshim/internal/winapi"
)

// Global run flag names.
const (
	measureArgName  = "measure"
	parallelArgName = "parallel"
	countArgName    = "count"
)

// Global uVM setting flag names.
const (
	cpusArgName                 = "cpus"
	memoryArgName               = "memory"
	allowOvercommitArgName      = "allow-overcommit"
	enableDeferredCommitArgName = "enable-deferred-commit"
	resourcePartitionArgName    = "resource-partition"
	useGCSArgName               = "gcs"
)

// Shared flag names.
const (
	execCommandLineArgName   = "exec"
	forwardStdoutArgName     = "fwd-stdout"
	forwardStderrArgName     = "fwd-stderr"
	outputHandlingArgName    = "output-handling"
	useTerminalArgName       = "tty"
	annotationsArgName       = "annotation"
	annotationsBase64ArgName = "annotation-base64"
	consolePipeArgName       = "console-pipe"
)

// Shared command flags.
var commonUVMFlags = []cli.Flag{
	cli.StringFlag{
		Name:  execCommandLineArgName,
		Usage: "Command to execute in the UVM.",
	},
	cli.BoolFlag{
		Name:  forwardStdoutArgName,
		Usage: "Whether stdout from the process in the UVM should be forwarded",
	},
	cli.BoolFlag{
		Name:  forwardStderrArgName,
		Usage: "Whether stderr from the process in the UVM should be forwarded",
	},
	cli.StringFlag{
		Name:  outputHandlingArgName,
		Usage: "Controls how output from UVM is handled. Use 'stdout' to print all output to stdout",
	},
	cli.BoolFlag{
		Name:  useTerminalArgName + ",t",
		Usage: "Create the process in the UVM with a TTY enabled",
	},
	cli.StringFlag{
		Name:  consolePipeArgName,
		Usage: "Named `pipe` for serial console output (which will be enabled)",
		// Set the console pipe in uvmboot by default, it helps with testing/debugging
		Value: `\\.\pipe\uvmpipe`,
	},
	cli.StringSliceFlag{
		Name: annotationsArgName + ",annot",
		Usage: "Annotation in the form of '`key=value`' to apply to the uVM. Use repeat instances to add multiple. " +
			"Annotations will be applied to the uVM BEFORE all other settings; " +
			"other flags will take precedence and override annotation settings. " +
			"There is no guaranteed precedence or ordering with respect to annotations.",
	},
	cli.StringSliceFlag{
		Name: annotationsBase64ArgName + ",annot64",
		Usage: "Base64-encoded annotation value, in the form '`key=base64`'. " +
			"See '" + annotationsArgName + "'.",
	},
}

type uvmRunFunc func(string) error

func main() {
	var debugLogs bool

	app := cli.NewApp()
	// app.Setup() checks if app.Writer is nil, but doesn't for app.ErrWriter
	app.ErrWriter = cli.ErrWriter
	app.Name = "uvmboot"
	app.Usage = "Boot a utility VM"

	app.Flags = []cli.Flag{
		cli.Uint64Flag{
			Name:  cpusArgName,
			Usage: "Number of CPUs on the UVM. Uses hcsshim default if not specified",
		},
		cli.UintFlag{
			Name:  memoryArgName,
			Usage: "Amount of memory on the UVM, in MB. Uses hcsshim default if not specified",
		},
		cli.BoolFlag{
			Name:  measureArgName,
			Usage: "Measure wall clock time of the UVM run",
		},
		cli.IntFlag{
			Name:  parallelArgName,
			Value: 1,
			Usage: "Number of UVMs to boot in parallel",
		},
		cli.IntFlag{
			Name:  countArgName,
			Value: 1,
			Usage: "Total number of UVMs to run",
		},
		cli.BoolFlag{
			Name:  allowOvercommitArgName,
			Usage: "Allow memory overcommit on the UVM",
		},
		cli.BoolFlag{
			Name:  enableDeferredCommitArgName,
			Usage: "Enable deferred commit on the UVM",
		},
		cli.BoolFlag{
			Name:        "debug",
			Usage:       "Increase logging verbosity",
			Destination: &debugLogs,
		},
		cli.StringFlag{
			Name:  resourcePartitionArgName,
			Usage: "Resource partition GUID to assign UVM to",
		},
		cli.BoolFlag{
			Name:  useGCSArgName,
			Usage: "Launch the GCS and perform requested operations via its RPC interface. Ignored for non-LCOW",
		},
	}

	app.Commands = []cli.Command{
		lcowCommand,
		wcowCommand,
		cwcowCommand,
	}

	app.Before = func(cCtx *cli.Context) error {
		// configure logging & tracing before any other validation
		trace.ApplyConfig(trace.Config{DefaultSampler: oc.DefaultSampler})
		trace.RegisterExporter(&oc.LogrusExporter{})

		logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

		lvl := logrus.WarnLevel
		if debugLogs {
			// as a debugging tool, opt for more logs over less
			lvl = logrus.TraceLevel
		}
		logrus.SetLevel(lvl)

		// log arguments individually to help with debug quoting/parsing issues
		if logrus.IsLevelEnabled(logrus.TraceLevel) {
			f := logrus.Fields{
				"app": cCtx.App.Name,
			}
			for i, s := range os.Args {
				f[fmt.Sprintf("arg%02d", i)] = s
			}
			logrus.WithFields(f).Trace("running command")
		}

		// start validation after logging is configured
		if !winapi.IsElevated() {
			return fmt.Errorf(cCtx.App.Name + " must be run in an elevated context")
		}

		return nil
	}

	// override default handler which calls [os.Exit] (via [cli.OsExiter]) for certain errors.
	app.ExitErrHandler = func(*cli.Context, error) {}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(app.ErrWriter, err)
		os.Exit(1)
	}
}

// func uVMCreateOptionsCommon[O uvm.OptionsLCOW | uvm.OptionsWCOW](
func uVMCreateOptionsCommon[O uvm.CreateOptions](
	ctx context.Context,
	cCtx *cli.Context,
	id, owner string,
) (o O, _ error) {
	spec := &specs.Spec{
		Annotations: parseAnnotationFlags(ctx, cCtx),
		Windows: &specs.Windows{
			HyperV: &specs.WindowsHyperV{},
		},
	}
	// check if LCOW
	switch any(o).(type) {
	case *uvm.OptionsLCOW:
		spec.Linux = &specs.Linux{}
	}

	if err := oci.ProcessAnnotations(ctx, spec); err != nil {
		return o, fmt.Errorf("unable to process annotations: %w", err)
	}

	opts, err := oci.SpecToUVMCreateOpts(ctx, spec, id, owner)
	if err != nil {
		return o, err
	}
	options, ok := opts.(O)
	if !ok {
		return o, fmt.Errorf("unexpected uVM options type: %T", opts)
	}

	cOpts := options.CommonOptions()

	if cCtx.GlobalIsSet(cpusArgName) {
		cOpts.ProcessorCount = int32(cCtx.GlobalUint64(cpusArgName))
	}
	if cCtx.GlobalIsSet(memoryArgName) {
		cOpts.MemorySizeInMB = cCtx.GlobalUint64(memoryArgName)
	}
	if cCtx.GlobalIsSet(allowOvercommitArgName) {
		cOpts.AllowOvercommit = cCtx.GlobalBool(allowOvercommitArgName)
	}
	if cCtx.GlobalIsSet(enableDeferredCommitArgName) {
		cOpts.EnableDeferredCommit = cCtx.GlobalBool(enableDeferredCommitArgName)
	}
	if cCtx.GlobalIsSet(enableDeferredCommitArgName) {
		cOpts.EnableDeferredCommit = cCtx.GlobalBool(enableDeferredCommitArgName)
	}
	if cCtx.GlobalIsSet(resourcePartitionArgName) {
		rpID, err := guid.FromString(cCtx.GlobalString(resourcePartitionArgName))
		if err != nil {
			return o, fmt.Errorf("parse resource partition GUID: %v", err)
		}
		cOpts.ResourcePartitionID = &rpID
	}

	if pipe := cCtx.String(consolePipeArgName); pipe != "" {
		cOpts.ConsolePipe = cCtx.String(consolePipeArgName)
	}

	return options, nil
}

// TODO: add a context here to propagate cancel/timeouts to runFunc uvm
// TODO: [runMany] can theoretically call runFunc multiple times on the same goroutine and starve others, fix that

func runMany(cCtx *cli.Context, runFunc uvmRunFunc) {
	parallelCount := cCtx.GlobalInt(parallelArgName)

	var wg sync.WaitGroup
	wg.Add(parallelCount)
	workChan := make(chan int)
	for i := 0; i < parallelCount; i++ {
		go func() {
			for i := range workChan {
				id := fmt.Sprintf("uvmboot-%d", i)
				if err := runFunc(id); err != nil {
					logrus.WithField("uvm-id", id).WithError(err).Error("failed to run UVM")
				}
			}
			wg.Done()
		}()
	}

	start := time.Now()
	for i := 0; i < cCtx.GlobalInt(countArgName); i++ {
		workChan <- i
	}

	close(workChan)
	wg.Wait()
	if cCtx.GlobalBool(measureArgName) {
		fmt.Println("Elapsed time:", time.Since(start))
	}
}

func unrecognizedError(name, value string) error {
	return fmt.Errorf("unrecognized value '%s' for option %s", name, value)
}
