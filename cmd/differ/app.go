package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	cli "github.com/urfave/cli/v2"
	"golang.org/x/sys/windows"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/hcsshim/internal/winapi"
)

/*
retstricted token
run on not default-desktop
 https://docs.microsoft.com/en-us/windows/win32/secauthz/restricted-tokens
*/

type beforeReExecFunc func(*cli.Context, *exec.Cmd) error

func wrapperApp() *cli.App {
	app := app()
	wrapper := &cli.App{}
	// copy the app
	*wrapper = *app
	wrapper.Commands = []*cli.Command{}
	return wrapper
}

var appCommands = []*cli.Command{
	sampleCommand,
	decompressCommand,
	convertCommand,
	wclayerCommand,
}

func app() *cli.App {
	app := &cli.App{
		Name:           "differ",
		Usage:          "Containerd stream processors for applying for Windows container (WCOW and LCOW) diffs and layers",
		Commands:       appCommands,
		ExitErrHandler: errHandler,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:   reExecFlagName,
				Usage:  "set after re-execing into this command with proper permissions and environment variables",
				Hidden: true,
			},
		},
	}
	return app
}

func errHandler(c *cli.Context, err error) {
	if err == nil {
		return
	}
	// reexec will return an exit code, so check for that edge case and
	if ee := (&exec.ExitError{}); errors.As(err, &ee) {
		err = cli.Exit("", ee.ExitCode())
	} else {
		s := c.App.Name
		if c.Command != nil && c.Command.Name != "" {
			s += " " + c.Command.Name
		}
		err = cli.Exit(fmt.Errorf("%s: %w", s, err), 1)
	}
	cli.HandleExitCoder(err)
}

// actionReExecWrapper returns a cli.ActionFunc that first checks if the re-exec flag
// is set, and if not, re-execs the command, with the flag set, and a stripped
// set of permissions. If r != nil, it will be run after creating the cmd to re-exec
func actionReExecWrapper(r beforeReExecFunc, f cli.ActionFunc) cli.ActionFunc {
	return func(c *cli.Context) error {
		// fmt.Printf("called with %s\n", os.Args)
		// fmt.Printf("is elevated? %t\n", winapi.IsElevated())
		// fmt.Printf("env? %q\n", os.Environ())

		printTokenPrivileges(windows.GetCurrentProcessToken())
		if c.Bool(reExecFlagName) {
			return f(c)
		}
		cmd := exec.CommandContext(c.Context, os.Args[0], append([]string{"-" + reExecFlagName}, os.Args[1:]...)...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stdout

		cmd.Env = []string{}
		for _, k := range []string{mediaTypeEnvVar, payloadPineEnvVar} {
			if v := os.Getenv(k); v != "" {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
		}

		var etoken windows.Token
		if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &etoken); err != nil {
			return fmt.Errorf("could not open process token: %w", err)
		}

		var token windows.Token
		if err := winapi.CreateRestrictedToken(
			etoken,
			winapi.TOKEN_DISABLE_MAX_PRIVILEGE,
			0, nil,
			0, nil,
			0, nil,
			&token,
		); err != nil {
			return fmt.Errorf("could not create restricted token: %w", err)
		}
		defer token.Close()
		printTokenPrivileges(token)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Token: syscall.Token(token),
		}
		// winio.EnableTokenPrivileges(token, []string{winio.SeBackupPrivilege, winio.SeRestorePrivilege})

		if r != nil {
			if err := r(c, cmd); err != nil {
				return fmt.Errorf("could not process cmd: %w", err)
			}
		}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("could not run command: %w", err)
		}
		return nil
	}
}

func printTokenPrivileges(token windows.Token) {
	b := make([]byte, 512)
	l := uint32(0)
	err := windows.GetTokenInformation(token, windows.TokenPrivileges, &b[0], uint32(len(b)), &l)
	if err == nil {
		fmt.Println("priv array len is", l)
		pv := (*windows.Tokenprivileges)(unsafe.Pointer(&b[0]))
		for _, o := range pv.AllPrivileges() {
			luid := (*uint64)(unsafe.Pointer(&o.Luid))
			s := winio.GetPrivilegeName(*luid)
			fmt.Printf("%s - %d\n", s, o.Attributes)
		}
	} else {
		fmt.Println("err was ", err, l)
	}

}
