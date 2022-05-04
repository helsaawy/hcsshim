package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	cli "github.com/urfave/cli/v2"

	"github.com/Microsoft/go-winio"
)

// todo: launch subcommands as process-isolated containers

const (
	mediaTypeEnvVar   = "STREAM_PROCESSOR_MEDIATYPE"
	payloadPineEnvVar = "STREAM_PROCESSOR_PIPE"
)

func main() {
	// because of ExitErrHandler, Run() should not return an error
	_ = app().Run(os.Args)
}

func app() *cli.App {
	app := &cli.App{
		Name:  "differ",
		Usage: "containerd differ stream processor utility for windows containers (WCOW and LCOW)",
		Commands: []*cli.Command{
			decompressCommand,
			convertCommand,
		},
		ExitErrHandler: func(c *cli.Context, err error) {
			if err == nil {
				return
			}
			s := c.App.Name
			if c.Command != nil && c.Command.Name != "" {
				s += ": " + c.Command.Name
			}
			cli.HandleExitCoder(cli.Exit(fmt.Errorf("%s: %w", s, err), 2))
		},
	}
	return app
}

func getMediaTypeEnvVar(ctx context.Context) ([]byte, error) {
	return readAllEnvPipe(ctx, mediaTypeEnvVar)
}

func getPayload(ctx context.Context) ([]byte, error) {
	return readAllEnvPipe(ctx, payloadPineEnvVar)
}

func readAllEnvPipe(ctx context.Context, env string) ([]byte, error) {
	n := os.Getenv(env)
	if n == "" {
		return nil, os.ErrNotExist
	}

	p, err := winio.DialPipeContext(ctx, n)
	if err != nil {
		return nil, fmt.Errorf("dial pipe %s from env var %v: %w", n, env, err)
	}
	defer p.Close()

	return ioutil.ReadAll(p)
}
