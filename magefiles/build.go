//go:build mage

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/mg"
)

// Build binaries, including for testing and release
//todo: general flags
// -race enable race detector (https://go.dev/doc/articles/race_detector)
// -blockprofile file write goroutine blocking statistics to file
// -mutexprofile file write mutex profile to file (https://pkg.go.dev/runtime#SetMutexProfileFraction)

// todo: -gcflags (go build -gcflags -help)
// see also https://go.dev/doc/diagnostics
// -N to disable optimizations
// -l disable inlining
// -m print optimization decisions, heap escapes and leaking params
// -c <count> concurrency during compilation
// -buildid id set build id
// -w debug type checking
// -smallframes reduce the size limit for stack allocated objects
// -spectre list enable spectre mitigations in list (all, index, ret)
// -nolocalimports reject local (relative) imports
// -dwarflocationlists add location lists to DWARF in optimized mode, for use with debuggers

// todo: -ldflags (go tool link -help)
// -s    disable symbol table
// -w    disable DWARF generation
// -n    dump symbol table

var (
	goBuildFlags     = []string{`-ldflags=-s -w`}
	goBuildTestFlags = []string{`-gcflags=all=-d=checkptr`}
)

type Release mg.Namespace

// Shim builds a release version of containerd-shim-runhcs-v1.
func (Release) Shim(ctx context.Context, version string) error {
	//todo: git tag? build date?
	return buildGoExe(ctx, "cmd/containerd-shim-runhcs-v1", cmdBin,
		varMap{"GOWORK": "off", "GOOS": "windows"}, varMap{"main.version": version}, nil, nil)
}

type Build mg.Namespace

func (Build) All(ctx context.Context) {
	mg.Deps(Build.Shim, Build.RunHCS, Build.AllTools,
		Build.GCS, Build.GCSTools, Build.WaitPaths)
}

// todo: move commands (into C:/ContainerPlat and into VM )

// Shim builds containerd-shim-runhcs-v1.
func (Build) Shim(ctx context.Context) error {
	return buildGoExe(ctx, "cmd/containerd-shim-runhcs-v1", cmdBin,
		varMap{"GOWORK": "off", "GOOS": "windows"}, nil, nil, nil)
}

func (Build) RunHCS(ctx context.Context) error {
	return buildGoExe(ctx, "cmd/runhcs", cmdBin,
		varMap{"GOWORK": "off", "GOOS": "windows"}, nil, nil, nil)
}

// Helper tools

func (Build) AllTools(ctx context.Context) {
	mg.Deps(Build.NCProxy, Build.UVMBoot, Build.DeviceUtil, Build.WCLayer,
		Build.Tar2Ext4, Build.Tar2Ext4Linux, Build.ShimDiag, Build.ZapDir)
}

func (Build) NCProxy(ctx context.Context) error {
	return buildGoExe(ctx, "cmd/ncproxy", cmdBin,
		varMap{"GOWORK": "off", "GOOS": "windows"}, nil, nil, nil)
}

func (Build) UVMBoot(ctx context.Context) error {
	return buildGoExe(ctx, "internal/tools/uvmboot", cmdBin,
		varMap{"GOWORK": "off", "GOOS": "windows"}, nil, nil, nil)
}

func (Build) DeviceUtil(ctx context.Context) error {
	return buildGoExe(ctx, "cmd/device-util", cmdBin,
		varMap{"GOWORK": "off", "GOOS": "windows"}, nil, nil, nil)
}

func (Build) WCLayer(ctx context.Context) error {
	return buildGoExe(ctx, "cmd/wclayer", cmdBin,
		varMap{"GOWORK": "off", "GOOS": "windows"}, nil, nil, nil)
}

func (Build) Tar2Ext4(ctx context.Context) error {
	return buildGoExe(ctx, "cmd/tar2ext4", cmdBin,
		varMap{"GOWORK": "off", "GOOS": "windows"}, nil, nil, nil)
}

func (Build) Tar2Ext4Linux(ctx context.Context) error {
	return buildGoExe(ctx, "cmd/tar2ext4", cmdBin,
		varMap{"GOWORK": "off", "GOOS": "linux"}, nil, nil, nil)
}

func (Build) ShimDiag(ctx context.Context) error {
	return buildGoExe(ctx, "cmd/shimdiag", cmdBin,
		varMap{"GOWORK": "off", "GOOS": "windows"}, nil, nil, nil)
}

func (Build) ZapDir(ctx context.Context) error {
	return buildGoExe(ctx, "internal/tools/zapdir", cmdBin,
		varMap{"GOWORK": "off", "GOOS": "windows"}, nil, nil, nil)
}

// Linux GCS components

func (Build) GCS(ctx context.Context) error {
	return buildGoExe(ctx, "cmd/gcs", cmdBin,
		varMap{"GOWORK": "off", "GOOS": "linux"}, nil, nil, nil)
}

func (Build) GCSTools(ctx context.Context) error {
	return buildGoExe(ctx, "cmd/gcstools", cmdBin,
		varMap{"GOWORK": "off", "GOOS": "linux"}, nil, nil, nil)
}

func (Build) WaitPaths(ctx context.Context) error {
	return buildGoExe(ctx, "cmd/hooks/wait-paths", cmdBin,
		varMap{"GOWORK": "off", "GOOS": "linux"}, nil, nil, nil)
}

// pkg should be relative to the rootDir, outDir can be relative or abs
func buildGoExe(ctx context.Context, pkg, outDir string, env, vars varMap, extraFlags, tags []string) error {
	mkdir(outDir)

	pkgPath := filepath.Join(rootDir, pkg)
	outPath := filepath.Join(outDir, filepath.Base(pkg))
	if env["GOOS"] == "windows" {
		outPath += ".exe"
	}

	args := make([]string, 0, len(goBuildFlags)+len(extraFlags)+6)
	args = append(args, "build", "-o", outPath)
	args = append(args, goBuildFlags...)
	args = append(args, extraFlags...)
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
	args = append(args, pkgPath)

	if _, err := Exec(ctx, goCmd(), args,
		execInDir(rootDir),
		execInheritEnv, // needs %LocalAppData% and other system variables
		execWithEnv(env),
		execVerbose,
	); err != nil {
		return fmt.Errorf("building pkg %q: %w", pkg, err)
	}

	if err := updateFileStamp(outPath); err != nil {
		// best effort, so log errors and continue
		log.Printf("updating %q timestamp and hash failed: %v", pkg, err)
	}
	return nil
}

// Test executables

type BuildTest mg.Namespace

func (BuildTest) All(ctx context.Context) {
	mg.Deps(BuildTest.CRIContainerd, BuildTest.Shim, BuildTest.RunHCS, BuildTest.Functional, Build.GCS)
}

func (BuildTest) CRIContainerd(ctx context.Context) error {
	return buildGoTestExe(ctx, "cri-container", testBin,
		varMap{"GOWORK": "off", "GOOS": "windows"}, nil, []string{"functional"})
}

func (BuildTest) Shim(ctx context.Context) error {
	return buildGoTestExe(ctx, "containerd-shim-runhcs-v1", testBin,
		varMap{"GOWORK": "off", "GOOS": "windows"}, nil, []string{"functional"})
}

func (BuildTest) RunHCS(ctx context.Context) error {
	return buildGoTestExe(ctx, "runhcs", testBin,
		varMap{"GOWORK": "off", "GOOS": "windows"}, nil, []string{"functional"})
}

func (BuildTest) Functional(ctx context.Context) error {
	return buildGoTestExe(ctx, "functional", testBin,
		varMap{"GOWORK": "off", "GOOS": "windows"}, nil, []string{"functional"})
}

func (BuildTest) GCS(ctx context.Context) error {
	return buildGoTestExe(ctx, "gcs", testBin,
		varMap{"GOWORK": "off", "GOOS": "linux"}, nil, []string{"functional"})
}

// pkg should be relative to testDir directory, outDir can be relative or abs
func buildGoTestExe(ctx context.Context, pkg, outDir string, env varMap, extraFlags, tags []string) error {
	mkdir(outDir)

	pkgPath := filepath.Join(testDir, pkg)
	// unlike `go build -o <path> ...`, `go test -c -o <path> ...` requires that path is
	// the target executable, and not the directory
	outPath := filepath.Join(outDir, filepath.Base(pkgPath)+".test")
	if env["GOOS"] == "windows" {
		outPath += ".exe"
	}

	args := make([]string, 0, len(goBuildTestFlags)+len(extraFlags)+6)
	args = append(args, "test", "-c", "-o", outPath)
	args = append(args, goBuildTestFlags...)
	args = append(args, extraFlags...)
	if len(tags) > 0 {
		args = append(args, "-tags="+strings.Join(tags, ","))
	}
	args = append(args, pkgPath)

	if _, err := Exec(ctx, goCmd(), args,
		execInDir(testDir),
		execInheritEnv, // needs %LocalAppData% and other system variables
		execWithEnv(env),
		execVerbose,
	); err != nil {
		return fmt.Errorf("building test exe %q: %w", pkgPath, err)
	}
	if err := updateFileStamp(outPath); err != nil {
		// best effort, so log errors and continue
		log.Printf("updating %q timestamp and hash failed: %v", pkgPath, err)
	}
	return nil
}

// todo: merge stamps together in one file?

// `go build` and `go test -c` do their own dependency mapping and incremental builds
// since go1.10 (see: https://pkg.go.dev/cmd/go#hdr-Build_and_test_caching).
// On Windows, this results in the go executable's modification time always changing after
// `go build` or `go test -c`, regardless of if the contents have.
// (Compare `dir <go exe>` and `go version -m <go exe> | where { $_ -match "vcs.time" }`)
// This makes it difficult to tell if a go file was rebuilt and requires rebuilding
// downstream components.
//
// So, build a cache of the exe's hash to see if it changed.
//
// Since go1.18, go embeds build information into binaries.
// That could be used in the future, when this project upgrades to that version.

// updateFileStamp records the file hash, only updating the value if it changed.
// This way, the file hash modification time can be used to detect if the file changed, rather
// then relying on the file timestamp itself.
// (`go build` updates the timestamp even if the output is unchanged.)
func updateFileStamp(file string) error {
	mkdir(depsDir)
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("computing hash of %q: %w", file, err)
	}

	// convert to hex string
	h.Size()
	sum := make([]byte, hex.EncodedLen(h.Size()))
	hex.Encode(sum, h.Sum(nil))

	depFile := getFileStampKey(file)
	if oldSum, err := os.ReadFile(depFile); err != nil || !bytes.Equal(oldSum, sum) {
		log.Printf("updating filestamp %q for %q\n", depFile, file)
		return os.WriteFile(depFile, sum, 0644)
	}
	return nil
}

// getFileStampKey returns the path to a hash file, whose modification time reflects when
// `file` was last updated.
func getFileStampKey(file string) string {
	return filepath.Join(depsDir, filepath.Base(file)+".sha256")
}
