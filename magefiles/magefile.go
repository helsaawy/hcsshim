//go:build mage

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Currently, there is no support to change the working directory using sh.Exec/Run/etc...
// Targets that need a specific working directory could [os.Chdir] at the begining of execution, but
// this is not thread safe, since multiple go-routines could cd and overlap
// The best option is to use [shellCmdsAnd] to string together "cd <target>: and the desired command(s)
//
// see: https://github.com/magefile/mage/issues/213

// todo: -gcflags (go build -gcflags -help)
// -N to disable optimizations
// -m print optimization decisions
// -race enable race detector
// -c <count> set to num cpus
// -buildid id set build id

// todo: -ldflags (go tool link -help)
// -race enable race detector
// -s    disable symbol table
// -w    disable DWARF generation
// -n    dump symbol table

var (
	goBuildFlags = []string{`-ldflags=-s -w`}
	goTestFlags  = []string{`-gcflags=all=-d=checkptr`}
)

var rootDir = func() string {
	// Can also find root with `go list -f '{{.Root}}' .`, but that assumes working directory
	// is a valid go pkg.
	// Embedding a dummy file as a embed.FS and accessing the fs.FileInfo would also work.
	_, r, _, ok := runtime.Caller(0)
	if !ok {
		panic("could not find root path for module")
	}
	return filepath.Dir(filepath.Dir(r))
}()

var (
	binDir  = filepath.Join(rootDir, "bin")
	cmdBin  = filepath.Join(binDir, "cmd")
	toolBin = filepath.Join(binDir, "tool")
	testBin = filepath.Join(binDir, "test")

	outDir      = filepath.Join(rootDir, "out")
	protobufDir = filepath.Join(rootDir, ".protobuf")

	testDir = filepath.Join(rootDir, "test")
)

func init() {
	if err := os.Chdir(rootDir); err != nil {
		panic(fmt.Errorf("could not change to root directory: %w", err))
	}
}

type varMap = map[string]string

var Default = ModRepo

var Aliases = map[string]interface{}{
	"mod":   ModRepo,
	"gen":   GoGenerate,
	"build": Build.Shim, // default build targe is the shim
	"shim":  Build.Shim,
}

// List is for debugging purposes
func List() {
	fmt.Println(shellCmd)
	p, _ := os.Getwd()
	fmt.Printf("go is %v\n%s\n", goCmd(), p)
}

//todo: call wsl
//todo: build tests
//todo: run unit tests

type Build mg.Namespace

// todo: move commands (into C:/ContainerPlat and into VM )

// Shim builds containerd-shim-runhcs-v1.
func (Build) Shim(version string) error {
	return buildGoExe(filepath.Join(rootDir, "cmd/containerd-shim-runhcs-v1"), cmdBin,
		varMap{"GOOS": "windows"}, nil, nil, nil)
}

type Release mg.Namespace

// Shim builds a release version of containerd-shim-runhcs-v1.
func (Release) Shim(version string) error {
	//todo: git tag? build date?
	return buildGoExe(filepath.Join(rootDir, "cmd/containerd-shim-runhcs-v1"), cmdBin,
		varMap{"GOOS": "windows"}, varMap{"main.version": version}, nil, nil)
}

func buildGoExe(pkg, outDir string, env, vars varMap, extraFlags, tags []string) error {
	if err := os.MkdirAll(outDir, 0750); err != nil {
		return fmt.Errorf("creating output directory %q: %w", outDir, err)
	}

	args := append(goBuildFlags, extraFlags...)
	if len(tags) > 0 {
		args = append(args, "-tags="+strings.Join(tags, ","))
	}
	if len(vars) > 0 {
		fs := make([]string, len(vars))
		for k, v := range vars {
			fs = append(fs, "-X "+k+"="+v)
		}
		args = append(args, "-ldflags="+strings.Join(fs, " "))
	}
	args = append(args, "-o", outDir, pkg)
	return sh.RunWithV(env, goCmd(), append([]string{"build"}, args...)...)
}

//
// file generation (go gen and protoc)
//

//todo: protobuf

// GoGenerate (re)generates files created by `//go:generate` directives.
func GoGenerate(_ context.Context) error {
	//todo: find files with generate directives and compare timestamps with dummy file?
	//todo: cache list of files found?
	return sh.RunWith(varMap{"GOOS": "windows"}, goCmd(), "generate", "-x", rootDir+"/...")
}

//
// go mod commands
//

// ModRepo calls [ModRoot] and [ModTest].
func ModRepo(ctx context.Context) {
	mg.SerialCtxDeps(ctx, ModRoot, ModTest)
}

// ModRoot runs `go mod tidy` and `go mod vendor` on the root module.
func ModRoot(ctx context.Context) error {
	return sh.Run(shellCmd, shellCmdsAnd(
		[]string{"cd", rootDir},
		[]string{goCmd(), "mod", "tidy", "-e", "-v"},
		[]string{goCmd(), "mod", "vendor", "-e"},
	)...)
}

// ModTest runs `go mod tidy` on `./test`.
func ModTest(_ context.Context) error {
	return sh.Run(shellCmd, shellCmdsAnd(
		[]string{"cd", testDir},
		[]string{goCmd(), "mod", "tidy", "-e", "-v"},
	)...)
}

//
// misc and cleanup
//

//todo: lint
//todo: PR prep (mod and lint)

// Clean removes (test) executables, and other output artifacts.
func Clean(_ context.Context) error {
	for _, dir := range []string{binDir, outDir, protobufDir} {
		if err := sh.Rm(dir); err != nil {
			return err
		}
	}
	return nil
}

// combines multiple []string{cmd, args...} together to passed directly to [shellCmd].
func shellCmdsAnd(cmds ...[]string) []string {
	n := len(shellFlags)
	for _, c := range cmds {
		n += len(c) + 1
	}
	n--

	cmd := make([]string, 0, n)
	cmd = append(cmd, shellFlags...)
	for i, c := range cmds {
		cmd = append(cmd, c...)
		if i != len(cmds)-1 {
			cmd = append(cmd, shellPipelineAnd)
		}
	}
	return cmd
}

func goCmd() string {
	p, err := exec.LookPath(mg.GoCmd())
	if err != nil {
		panic(fmt.Sprintf("invalid go executable; "+
			"specify the location with the '%s' environment variable: %v",
			mg.GoCmdEnv, err))
	}
	return p
}
