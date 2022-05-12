package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

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
func actionReExecWrapper(f cli.ActionFunc, opts ...reExecOpts) cli.ActionFunc {
	conf := reExecConfig{}
	var confErr error // cant return an error here, so punt error checking till action action
	opts = append(opts, defaultPrivileges)
	for _, o := range opts {
		if confErr := o(&conf); confErr != nil {
			break
		}
	}
	return func(c *cli.Context) error {
		if confErr != nil {
			return fmt.Errorf("could not properly initialize re-exec config: %w", confErr)
		}

		if c.Bool(reExecFlagName) {
			// fmt.Printf("called with %s\n", os.Args)
			fmt.Printf("is elevated? %t\n", winapi.IsElevated())
			// fmt.Printf("env? %q\n", os.Environ())
			printTokenPrivileges(windows.GetCurrentProcessToken())
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
		if err := windows.OpenProcessToken(windows.CurrentProcess(),
			windows.TOKEN_DUPLICATE|windows.TOKEN_ASSIGN_PRIMARY|windows.TOKEN_QUERY|
				windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_WRITE,
			&etoken,
		); err != nil {
			return fmt.Errorf("could not open process token: %w", err)
		}

		deleteLUIDs, err := privilegesToDelete(etoken, conf.keepPrivleges)
		if err != nil {
			return fmt.Errorf("could not get privileges to delete: %w", err)
		}

		var token windows.Token
		if err := winapi.CreateRestrictedToken(
			etoken,
			0,   // flags
			nil, // SIDs to disable
			deleteLUIDs,
			nil, // SIDs to restrict
			&token,
		); err != nil {
			return fmt.Errorf("could not create restricted token: %w", err)
		}
		defer token.Close()
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Token: syscall.Token(token),
		}

		fmt.Println("about to start", cmd.String())
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("could not start command: %w", err)
		}
		return cmd.Wait()
	}
}

func printTokenPrivileges(token windows.Token) {
	pv, err := winapi.GetTokenPrivileges(token)
	if err == nil {
		fmt.Println("priv array len is", pv.PrivilegeCount)
		for _, o := range pv.AllPrivileges() {
			s := winio.GetPrivilegeName(uint64(winapi.LUIDToInt(o.Luid)))
			fmt.Printf("%s - %d\n", s, o.Attributes)
		}
	} else {
		fmt.Println("err was ", err)
	}
}
