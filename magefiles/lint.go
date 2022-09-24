//go:build mage

package main

import (
	"context"
	"path/filepath"

	"github.com/magefile/mage/mg"
)

var lintFlags = []string{
	"--timeout=2m",
	"--max-issues-per-linter=0",
	"--max-same-issues=0",
	"--modules-download-mode=readonly",
	"--config=" + filepath.Join(rootDir, ".golangci.yml"),
}

type Lint mg.Namespace

func (Lint) Repo(ctx context.Context) {
	mg.SerialCtxDeps(ctx, Lint.Root, Lint.Test, Lint.Linux)
}

func (Lint) Root(ctx context.Context) error {
	return lint(ctx, rootDir, "windows")
}

func (Lint) Test(ctx context.Context) error {
	return lint(ctx, testDir, "windows")
}

func (Lint) Linux(ctx context.Context) {
	mg.SerialCtxDeps(ctx, Lint.RootLinux, Lint.TestLinux)
}

func (Lint) RootLinux(ctx context.Context) error {
	return lint(ctx, rootDir, "linux",
		"cmd/gcs", "cmd/gcstools",
		"internal/guest", "internal/tools",
		"ext4", "pkg", "magefiles")
}

func (Lint) TestLinux(ctx context.Context) error {
	return lint(ctx, testDir, "linux")
}

func lint(ctx context.Context, dir, goos string, paths ...string) error {
	// todo: check if linter exists, and install otherwise
	args := make([]string, 0, len(lintFlags)+len(paths)+2)
	args = append(args, "run")
	args = append(args, lintFlags...)
	if mg.Verbose() {
		args = append(args, "--verbose")
	}
	args = append(args, paths...)

	if _, err := Exec(ctx, "golangci-lint", args,
		// if _, err := Exec(ctx, "cmd", []string{"/c", "set"},
		execInDir(dir),
		execInheritEnv, // golangci-lint needs %LocalAppData% for caching
		execWithEnv(varMap{
			"PATH":   addToPath(toolBin),
			"GOOS":   goos,
			"GOWORK": "off",
		}),
		execVerbose,
	); err != nil {
		return err
	}
	return nil
}
