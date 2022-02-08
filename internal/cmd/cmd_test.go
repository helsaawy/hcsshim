//go:build windows
// +build windows

package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Microsoft/hcsshim/internal/cow"
	hcsschema "github.com/Microsoft/hcsshim/internal/hcs/schema2"
)

type localProcessHost struct {
}

var _ cow.ProcessHost = &localProcessHost{}

type localProcess struct {
	p                     *os.Process
	state                 *os.ProcessState
	ch                    chan struct{}
	stdin, stdout, stderr *os.File
}

var _ cow.Process = &localProcess{}

func (h *localProcessHost) OS() string {
	return "windows"
}

func (h *localProcessHost) IsOCI() bool {
	return false
}

func (h *localProcessHost) CreateProcess(ctx context.Context, cfg interface{}) (_ cow.Process, err error) {
	params := cfg.(*hcsschema.ProcessParameters)
	lp := &localProcess{ch: make(chan struct{})}
	defer func() {
		if err != nil {
			lp.Close()
		}
	}()
	var stdin, stdout, stderr *os.File
	if params.CreateStdInPipe {
		stdin, lp.stdin, err = os.Pipe()
		if err != nil {
			return nil, err
		}
		defer stdin.Close()
	}
	if params.CreateStdOutPipe {
		lp.stdout, stdout, err = os.Pipe()
		if err != nil {
			return nil, err
		}
		defer stdout.Close()
	}
	if params.CreateStdErrPipe {
		lp.stderr, stderr, err = os.Pipe()
		if err != nil {
			return nil, err
		}
		defer stderr.Close()
	}
	path := strings.Split(params.CommandLine, " ")[0] // should be fixed for non-test use...
	if ppath, err := exec.LookPath(path); err == nil {
		path = ppath
	}
	lp.p, err = os.StartProcess(path, nil, &os.ProcAttr{
		Files: []*os.File{stdin, stdout, stderr},
		Sys: &syscall.SysProcAttr{
			CmdLine: params.CommandLine,
		},
	})
	if err != nil {
		return nil, err
	}
	go func() {
		lp.state, _ = lp.p.Wait()
		close(lp.ch)
	}()
	return lp, nil
}

func (p *localProcess) Close() error {
	if p.p != nil {
		_ = p.p.Release()
	}
	if p.stdin != nil {
		p.stdin.Close()
	}
	if p.stdout != nil {
		p.stdout.Close()
	}
	if p.stderr != nil {
		p.stderr.Close()
	}
	return nil
}

func (p *localProcess) CloseStdin(ctx context.Context) error {
	return p.stdin.Close()
}

func (p *localProcess) CloseStdout(ctx context.Context) error {
	return p.stdout.Close()
}

func (p *localProcess) CloseStderr(ctx context.Context) error {
	return p.stderr.Close()
}

func (p *localProcess) ExitCode() (int, error) {
	select {
	case <-p.ch:
		return p.state.ExitCode(), nil
	default:
		return -1, errors.New("not exited")
	}
}

func (p *localProcess) Kill(ctx context.Context) (bool, error) {
	return true, p.p.Kill()
}

func (p *localProcess) Signal(ctx context.Context, _ interface{}) (bool, error) {
	return p.Kill(ctx)
}

func (p *localProcess) Pid() int {
	return p.p.Pid
}

func (p *localProcess) ResizeConsole(ctx context.Context, x, y uint16) error {
	return errors.New("not supported")
}

func (p *localProcess) Stdio() (io.Writer, io.Reader, io.Reader) {
	return p.stdin, p.stdout, p.stderr
}

func (p *localProcess) Wait() error {
	<-p.ch
	return nil
}

func TestCmdExitCode(t *testing.T) {
	cmd := Command(&localProcessHost{}, "cmd", "/c", "exit", "/b", "64")
	err := cmd.Run()
	if e, ok := err.(*ExitError); !ok || e.ExitCode() != 64 {
		t.Fatal("expected exit code 64, got ", err)
	}
}

func TestCmdOutput(t *testing.T) {
	cmd := Command(&localProcessHost{}, "cmd", "/c", "echo", "hello")
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "hello\r\n" {
		t.Fatalf("got %q", string(output))
	}
}

func TestCmdContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	cmd := CommandContext(ctx, &localProcessHost{}, "cmd", "/c", "pause")
	r, w := io.Pipe()
	cmd.Stdin = r
	cmd.RegisterAfterExitFun(func(_ context.Context) error { return w.Close() })

	err := cmd.Start()
	if err != nil {
		t.Fatal(err)
	}

	err = cmd.Wait()
	if e, ok := err.(*ExitError); !ok || e.ExitCode() != 1 || ctx.Err() == nil {
		t.Fatal(err)
	}
}

func TestCmdStdin(t *testing.T) {
	cmd := Command(&localProcessHost{}, "findstr", "x*")
	cmd.Stdin = bytes.NewBufferString("testing 1 2 3")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "testing 1 2 3\r\n" {
		t.Fatalf("got %q", string(out))
	}
}

func TestCmdStdinBlocked(t *testing.T) {
	cmd := Command(&localProcessHost{}, "cmd", "/c", "pause")
	r, w := io.Pipe()
	defer r.Close()
	go func() {
		b := []byte{'\n'}
		_, _ = w.Write(b)
	}()
	cmd.Stdin = r
	_, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
}

func TestCmdAfterExitFun(t *testing.T) {
	cmd := Command(&localProcessHost{}, "cmd", "/c")

	c := make(chan struct{})
	cmd.RegisterAfterExitFun(func(_ context.Context) error {
		close(c)
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	err := cmd.Start()
	if err != nil {
		t.Fatal(err)
	}

	// call cmd.Wait to make sure after funs are called
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
		close(done)
	}()

	err = cmd.Process.Wait()
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-c:
		// if they both finish at the same time, it is undefined which case is chosen...
	case <-cmd.allDoneCh:
		t.Fatalf("after exit did not finish before cmd.Wait returned")
	}

	// still check for errors during cmd.Wait():
	err = <-done
	if err != nil {
		t.Fatalf("cmd failed: %v", err)
	}
}

func TestCmdAfterExitFunRegistration(t *testing.T) {
	cmd := Command(&localProcessHost{}, "cmd", "/c", "echo", "hello")

	l := len(cmd.afterExitFuns)
	cmd.RegisterAfterExitFun(func(_ context.Context) error {
		return nil
	})
	if len(cmd.afterExitFuns) != l+1 {
		t.Fatalf("function registration failed")
	}

	err := cmd.Start()
	if err != nil {
		t.Fatalf("cmd Run failed: %v", err)
	}

	cmd.RegisterAfterExitFun(func(_ context.Context) error {
		return errors.New("this error should never be raised")
	})

	if len(cmd.afterExitFuns) != l+1 {
		t.Fatalf("function should not have been registered")
	}
}

type stuckIoProcessHost struct {
	cow.ProcessHost
}

type stuckIoProcess struct {
	cow.Process
	stdin, pstdout, pstderr *io.PipeWriter
	pstdin, stdout, stderr  *io.PipeReader
}

func (h *stuckIoProcessHost) CreateProcess(ctx context.Context, cfg interface{}) (cow.Process, error) {
	p, err := h.ProcessHost.CreateProcess(ctx, cfg)
	if err != nil {
		return nil, err
	}
	sp := &stuckIoProcess{
		Process: p,
	}
	sp.pstdin, sp.stdin = io.Pipe()
	sp.stdout, sp.pstdout = io.Pipe()
	sp.stderr, sp.pstderr = io.Pipe()
	return sp, nil
}

func (p *stuckIoProcess) Stdio() (io.Writer, io.Reader, io.Reader) {
	return p.stdin, p.stdout, p.stderr
}

func (p *stuckIoProcess) Close() error {
	p.stdin.Close()
	p.stdout.Close()
	p.stderr.Close()
	return p.Process.Close()
}

func TestCmdStuckIo(t *testing.T) {
	cmd := Command(&stuckIoProcessHost{&localProcessHost{}}, "cmd", "/c", "echo", "hello")
	cmd.CopyAfterExitTimeout = time.Millisecond * 200
	_, err := cmd.Output()
	if err != io.ErrClosedPipe {
		t.Fatal(err)
	}
}

// check that io Copy will wait indefintely if pipes are not closed
func TestCmdStuckStdoutNotClosed(t *testing.T) {
	cmd := Command(&stuckIoProcessHost{&localProcessHost{}}, "cmd", "/c")
	r, w := io.Pipe()
	defer r.Close()
	cmd.Stdout = w

	done := make(chan error)
	go func() {
		done <- cmd.Run()
		close(done)
	}()

	tr := time.NewTimer(250 * time.Millisecond) // give the cmd a chance to finish running
	defer tr.Stop()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("cmd run failed: %v", err)
		}
		t.Fatal("command should have blocked indefinitely")
	case <-tr.C:
	}
}

func TestCmdStuckStdoutClosed(t *testing.T) {
	cmd := Command(&stuckIoProcessHost{&localProcessHost{}}, "cmd", "/c")
	r, w := io.Pipe()
	defer r.Close()
	cmd.Stdout = w
	cmd.RegisterAfterExitFun(func(ctx context.Context) error {
		p := cmd.Process.(*stuckIoProcess)
		return p.stdout.Close()
	})

	done := make(chan error)
	go func() {
		done <- cmd.Run()
		close(done)
	}()

	tr := time.NewTimer(250 * time.Millisecond)
	defer tr.Stop()
	select {
	case err := <-done:
		if err != io.ErrClosedPipe {
			t.Fatalf("cmd run failed: %v", err)
		}
	case <-tr.C:
		t.Fatal("command did not exit")
	}
}
