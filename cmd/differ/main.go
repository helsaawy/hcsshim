package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"github.com/sirupsen/logrus"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/go-winio/pkg/etw"
	"github.com/Microsoft/go-winio/pkg/etwlogrus"
	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/Microsoft/hcsshim/cmd/differ/payload"
)

// todo: launch subcommands as process-isolated containers

const (
	mediaTypeEnvVar   = "STREAM_PROCESSOR_MEDIATYPE"
	payloadPineEnvVar = "STREAM_PROCESSOR_PIPE"

	reExecFlagName = "reexec"
)

func main() {
	// Run() should not return an error because of ExitErrHandler, but just in case ...
	if err := app().Run(os.Args); err != nil {
		log.New(os.Stderr, "", 0).Fatal(err)
	}
}

func getMediaType(ctx context.Context) string {
	return os.Getenv(mediaTypeEnvVar)
}

func getPayload(ctx context.Context, p payload.FromAny) error {
	b, err := readAllEnvPipe(ctx, payloadPineEnvVar)
	if err != nil || b == nil {
		return err
	}

	a := &types.Any{}
	if err := proto.Unmarshal(b, a); err != nil {
		return fmt.Errorf("proto.Unmarshal(): %w", err)
	}
	return p.FromAny(a)
}

func readAllEnvPipe(ctx context.Context, env string) ([]byte, error) {
	n := os.Getenv(env)
	if n == "" {
		return nil, nil
	}

	p, err := winio.DialPipeContext(ctx, n)
	if err != nil {
		return nil, fmt.Errorf("dial pipe %s from env var %v: %w", n, env, err)
	}
	defer p.Close()

	return ioutil.ReadAll(p)
}

func setupLogging() {
	f := func(guid.GUID, etw.ProviderState, etw.Level, uint64, uint64, uintptr) {}
	provider, err := etw.NewProvider("ContainerD", f)
	if err != nil {
		logrus.Error(err)
	} else {
		if hook, err := etwlogrus.NewHookFromProvider(provider); err == nil {
			logrus.AddHook(hook)
		} else {
			logrus.Error(err)
		}
	}
	logrus.SetOutput(ioutil.Discard)
	logrus.SetLevel(logrus.DebugLevel)
}
