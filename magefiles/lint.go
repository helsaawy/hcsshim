//go:build mage

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/magefile/mage/mg"
)

var lintFlags = []string{
	"--timeout=2m",
	"--max-issues-per-linter=0",
	"--max-same-issues=0",
	"--modules-download-mode=readonly",
	// use getRootDir instead of rootDir since the latter may not be initialized  yet
	"--config=" + filepath.Join(getRootDir(), ".golangci.yml"),
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
	mg.SerialCtxDeps(ctx, Lint.LinuxRoot, Lint.LinuxTest)
}

func (Lint) LinuxRoot(ctx context.Context) error {
	//todo: add `magefiles/...` when this branch is merged into main
	return lint(ctx, rootDir, "linux",
		"cmd/gcs/...", "cmd/gcstools/...",
		"internal/guest/...", "internal/tools/...",
		"ext4/...", "pkg/...")
}

func (Lint) LinuxTest(ctx context.Context) error {
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

// Install installs the latest version of golangci-lint, if it is not found in the path.
func (Lint) Install(ctx context.Context) error {
	mkdir(toolBin)
	baseURL, err := url.Parse("http://github.com/golangci/golangci-lint/releases")
	if err != nil {
		return fmt.Errorf("parsing base URL:%w", err)
	}
	body, err := get(ctx, baseURL.JoinPath("latest").String(), varMap{"Accept": "application/json"})
	if err != nil {
		return err
	}
	var jb map[string]interface{}
	if err := json.Unmarshal(body, &jb); err != nil {
		return fmt.Errorf("json unmarshal for %q: %w", string(body), err)
	}
	verRaw, ok := jb["tag_name"]
	if !ok {
		return fmt.Errorf("json body did not contain field %q: %v", "tag_name", jb)
	}
	ver := verRaw.(string)
	log.Printf("installing golangci-lint version: %s", ver)
	f := "golangci-lint-" + strings.TrimPrefix(ver, "v") + "-" + runtime.GOOS + "-" + runtime.GOARCH + archiveExt
	out := filepath.Join(outDir, f)

	if err := download(ctx, baseURL.JoinPath("download", ver, f).String(), nil, out); err != nil {
		return fmt.Errorf("downloading linter: %w", err)
	}
	// defer os.Remove(out)

	return tarExtract(toolBin, out, args("--strip-components=1"), args("*/golangci-lint"+binaryExt))
}

// linux can have either curl, wget, or neither; use these as a catch all

func download(ctx context.Context, url string, headers varMap, file string) error {
	f, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("output file %q creation: %w", file, err)
	}
	defer f.Close()

	body, err := getRC(ctx, url, headers)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer body.Close()

	n, err := io.Copy(f, body)
	if err != nil {
		return fmt.Errorf("copying to file %q: %q", file, err)
	}
	log.Printf("downloaded %d bytes from %q to %q", n, url, file)
	return nil
}

func get(ctx context.Context, url string, headers varMap) ([]byte, error) {
	body, err := getRC(ctx, url, headers)
	if err != nil {
		return nil, err
	}

	b, err := io.ReadAll(body)
	body.Close()
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	return b, nil
}

func getRC(ctx context.Context, url string, headers varMap) (io.ReadCloser, error) {
	r, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		r.Header.Add(k, v)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		if b, err := io.ReadAll(resp.Body); err == nil {
			return nil, fmt.Errorf("error code %d: %s", resp.StatusCode, string(b))
		}
		return nil, fmt.Errorf("error code %d", resp.StatusCode)
	}
	return resp.Body, nil
}
