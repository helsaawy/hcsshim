//go:build windows

package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"go.opencensus.io/trace"

	"github.com/Microsoft/hcsshim/internal/oc"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/Microsoft/hcsshim/internal/winapi"
)

// Global flag names.
const (
	cpusArgName                 = "cpus"
	memoryArgName               = "memory"
	allowOvercommitArgName      = "allow-overcommit"
	enableDeferredCommitArgName = "enable-deferred-commit"
	measureArgName              = "measure"
	parallelArgName             = "parallel"
	countArgName                = "count"
	useGCSArgName               = "gcs"
)

// Shared command flag names.
const (
	execCommandLineArgName = "exec"
	forwardStdoutArgName   = "fwd-stdout"
	forwardStderrArgName   = "fwd-stderr"
	outputHandlingArgName  = "output-handling"
	useTerminalArgName     = "tty"
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
		Usage: "create the process in the UVM with a TTY enabled",
	},
}

type uvmRunFunc func(string) error

func main() {
	var debugLogs bool
	var traceLogs bool

	app := cli.NewApp()
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
			Usage:       "Enable debug logs",
			Destination: &debugLogs,
		},
		cli.BoolFlag{
			Name:        "trace",
			Usage:       "Enable trace logs (implies debug logs)",
			Destination: &traceLogs,
		},
		cli.BoolFlag{
			Name:  useGCSArgName,
			Usage: "Launch the GCS and perform requested operations via its RPC interface. Currently LCOW only",
		},
	}

	app.Commands = []cli.Command{
		lcowCommand,
		wcowCommand,
	}

	app.Before = func(cCtx *cli.Context) error {
		if !winapi.IsElevated() {
			return fmt.Errorf(cCtx.App.Name + " must be run in an elevated context")
		}

		// configure logging/tracing
		trace.ApplyConfig(trace.Config{DefaultSampler: oc.DefaultSampler})
		trace.RegisterExporter(&oc.LogrusExporter{})

		logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

		lvl := logrus.WarnLevel
		if traceLogs {
			if debugLogs {
				logrus.Warn(`"debug" and "trace" flags are mutually exclusive`)
			}
			lvl = logrus.TraceLevel
		} else if debugLogs {
			lvl = logrus.DebugLevel
		}
		logrus.SetLevel(lvl)

		logrus.WithField("args", fmt.Sprintf("%#+v", os.Args)).Tracef("running %s command", cCtx.App.Name)
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func setGlobalOptions(c *cli.Context, options *uvm.Options) {
	if c.GlobalIsSet(cpusArgName) {
		options.ProcessorCount = int32(c.GlobalUint64(cpusArgName))
	}
	if c.GlobalIsSet(memoryArgName) {
		options.MemorySizeInMB = c.GlobalUint64(memoryArgName)
	}
	if c.GlobalIsSet(allowOvercommitArgName) {
		options.AllowOvercommit = c.GlobalBool(allowOvercommitArgName)
	}
	if c.GlobalIsSet(enableDeferredCommitArgName) {
		options.EnableDeferredCommit = c.GlobalBool(enableDeferredCommitArgName)
	}
}

// TODO: add a context here to propagate cancel/timeouts to runFunc uvm
// TODO: [runMany] can theoretically call runFunc multiple times on the same goroutine and starve others, fix that

func runMany(c *cli.Context, runFunc uvmRunFunc) {
	parallelCount := c.GlobalInt(parallelArgName)

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
	for i := 0; i < c.GlobalInt(countArgName); i++ {
		workChan <- i
	}

	close(workChan)
	wg.Wait()
	if c.GlobalBool(measureArgName) {
		fmt.Println("Elapsed time:", time.Since(start))
	}
}

func unrecognizedError(name, value string) error {
	return fmt.Errorf("unrecognized value '%s' for option %s", name, value)
}
