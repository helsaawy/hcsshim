//go:build windows
// +build windows

package devices

import (
	"context"
	"io/ioutil"
	"net"

	"github.com/pkg/errors"

	"github.com/Microsoft/hcsshim/internal/cmd"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/uvm"
)

func execModprobeInstallDriver(ctx context.Context, vm *uvm.UtilityVM, driverDir string) error {
	p, l, err := cmd.CreateNamedPipeListener()
	if err != nil {
		return err
	}
	defer l.Close()

	var stderrOutput string
	errChan := make(chan error)

	go readAllPipeOutput(l, errChan, &stderrOutput)

	args := []string{
		"/bin/install-drivers",
		driverDir,
	}
	req := &cmd.CmdProcessRequest{
		Args:   args,
		Stderr: p,
	}

	exitCode, execErr := cmd.ExecInUvm(ctx, vm, req)

	// wait to finish parsing stdout results
	select {
	case err := <-errChan:
		if err != nil {
			return errors.Wrap(err, execErr.Error())
		}
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), execErr.Error())
	}

	if execErr != nil && execErr != errNoExecOutput {
		return errors.Wrapf(execErr, "failed to install driver %s in uvm with exit code %d: %v", driverDir, exitCode, stderrOutput)
	}

	log.G(ctx).WithField("added drivers", driverDir).Debug("installed drivers")
	return nil
}

// readAllPipeOutput is a helper function that connects to a listener and attempts to
// read the connection's entire output. Resulting output is returned as a string
// in the `result` param. The `errChan` param is used to propagate an errors to
// the calling function.
func readAllPipeOutput(l net.Listener, errChan chan<- error, result *string) {
	defer close(errChan)
	c, err := l.Accept()
	if err != nil {
		errChan <- errors.Wrapf(err, "failed to accept named pipe")
		return
	}
	bytes, err := ioutil.ReadAll(c)
	if err != nil {
		errChan <- err
		return
	}

	*result = string(bytes)

	if len(*result) == 0 {
		errChan <- errNoExecOutput
		return
	}

	errChan <- nil
}
