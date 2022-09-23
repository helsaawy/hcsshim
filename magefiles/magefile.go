//go:build mage

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Microsoft/hcsshim/ext4/tar2ext4"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/magefile/mage/target"
	"github.com/u-root/u-root/pkg/cpio"
)

// Currently, there is no support to change the working directory using sh.Exec/Run/etc...
// Targets that need a specific working directory could [os.Chdir] at the begining of execution, but
// this is not thread safe, since multiple go-routines could cd and overlap
// Currently, using  [shellCmdsAnd] to string together "cd <target> &&" with the desired command(s)
//TODO: create custom exec that offers same printing as sh.Exec, but allows changing working dir
//
// see: https://github.com/magefile/mage/issues/213

const nopTargetFmt = "no work to do for %q\n"

var errMissingDep = errors.New("missing dependency")

// rootDir the full path to the repo.
// mage runs with magefiles as the working directory, but the compiled binary can be put
// anywhere, so no guarantee as too location.
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
	cmdBin  = filepath.Join(binDir, "cmd")  // binaries from ./cmd and ./internal/tools
	testBin = filepath.Join(binDir, "test") // binaries from ./test
	toolBin = filepath.Join(binDir, "tool") // dependencies (ie, protobuf)

	outDir      = filepath.Join(rootDir, "out")  // intermediary products and outputs
	depsDir     = filepath.Join(rootDir, "deps") // dependency tracking
	protobufDir = filepath.Join(rootDir, ".protobuf")

	testDir = filepath.Join(rootDir, "test")
)

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
	lintFlags        = []string{
		"--timeout=2m",
		"--max-issues-per-linter=0",
		"--max-same-issues=0",
		"--modules-download-mode=readonly",
		"--config=" + filepath.Join(rootDir, ".golangci.yml"),
	}
)

func init() {
	if err := os.Chdir(rootDir); err != nil {
		panic(fmt.Errorf("could not change to root directory: %w", err))
	}
}

type varMap = map[string]string

var Default = ModRepo

var Aliases = map[string]interface{}{
	"pr":     Validate,
	"mod":    ModRepo,
	"gen":    GoGenerate,
	"delta":  DeltaTarGz,
	"rootfs": RootfsVHD,
	"init":   Initramfs,
	"lint":   Lint.Repo,

	// default build target is the shim
	"build": Build.Shim,
	"shim":  Build.Shim,

	// default BuildTest target is All
	"buildtest": BuildTest.All,
	"critest":   BuildTest.CRIContainerd,
	"functest":  BuildTest.Functional,
}

//todo: protobuf
//todo: call wsl for init and vsock
//todo: rego support to gcs, shim, and test exes
//todo: pull/build docker images for cri-containerd\test-images
//todo: run unit tests
//todo: mg.Verbose-aware logger
//todo: dependencies (golangci-lint, docker, protoc)

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
	mg.Deps(mg.F(mkdir, outDir))

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
	mg.Deps(mg.F(mkdir, outDir))

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

// updateFileStamp records the file hash, only updating the value if it changed.
// This way, the file hash modification time can be used to detect if the file changed, rather
// then relying on the file timestamp itself.
// (`go build` updates the timestamp even if the output is unchanged.)
func updateFileStamp(file string) error {
	mg.Deps(mg.F(mkdir, depsDir))
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

//
// general file and artifact generation (go gen and protoc)
//

var rootfsVHDPath = filepath.Join(outDir, "rootfs.vhd")

func RootfsVHD(_ context.Context, baseTar string) error {
	mg.Deps(mg.F(mkdir, filepath.Dir(rootfsVHDPath)), mg.F(RootfsTar, baseTar))
	build, err := target.Path(rootfsVHDPath, rootfsTarPath)
	if !build {
		if mg.Verbose() {
			log.Printf(nopTargetFmt, rootfsVHDPath)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("%v: %w", errMissingDep, err)
	}

	rootTar, err := os.Open(rootfsTarPath)
	if err != nil {
		return fmt.Errorf("open rootfs tar file %q: %w", rootfsTarPath, err)
	}
	defer rootTar.Close()

	rootVHD, err := os.Create(rootfsVHDPath)
	if err != nil {
		return fmt.Errorf("create rootfs VHD file %q: %w", rootfsVHDPath, err)
	}
	defer rootVHD.Close()

	if err = tar2ext4.Convert(rootTar, rootVHD, tar2ext4.AppendVhdFooter); err != nil {
		return fmt.Errorf("converting rootfs tar file %q to VHD %q: %w",
			rootfsTarPath, rootfsVHDPath, err)
	}

	if mg.Verbose() {
		log.Printf("created rootfs VHD file %q\n", rootfsVHDPath)
	}
	return nil
}

var initramfsPath = filepath.Join(outDir, "initrd.img")

func Initramfs(_ context.Context, baseTar string) error {
	mg.Deps(mg.F(mkdir, filepath.Dir(initramfsPath)), mg.F(RootfsTar, baseTar))
	build, err := target.Path(initramfsPath, rootfsTarPath)
	if !build {
		if mg.Verbose() {
			log.Printf(nopTargetFmt, initramfsPath)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("%v: %w", errMissingDep, err)
	}

	rootTar, cleanup, err := openTarFile(rootfsTarPath)
	if err != nil {
		return fmt.Errorf("open rootfs tar file %q: %w", rootfsTarPath, err)
	}
	defer cleanup()

	f, err := os.Create(initramfsPath)
	if err != nil {
		return fmt.Errorf("create initramfs file %q: %w", initramfsPath, err)
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	defer gw.Close()

	img := cpio.Newc.Writer(gw)
	// u-root doesnt support converting filesystem info to records on windows
	// plus, files are in a tar archive, so create records on the fly
	if err := copyTarToCPIO(img, *rootTar); err != nil {
		return fmt.Errorf("copying rootfs tar %q to initramfs %q: %w",
			rootfsTarPath, initramfsPath, err)
	}

	if err := cpio.WriteTrailer(img); err != nil {
		return fmt.Errorf("writing CPIO trailer to %q: %w", initramfsPath, err)
	}
	if mg.Verbose() {
		log.Printf("created initramfs file %q\n", initramfsPath)
	}
	return nil
}

func copyTarToCPIO(dst cpio.RecordWriter, src tar.Reader) error {
	var inode uint64
	for {
		hdr, err := src.Next()
		switch err {
		case nil:
		case io.EOF:
			return nil
		default:
			return err
		}
		r := cpio.Record{}

		switch hdr.Typeflag {
		case tar.TypeReg:
		case tar.TypeDir:
		case tar.TypeSymlink:
			// cpio.Symlink()
		default:
			return fmt.Errorf("unsupported tar type %x", hdr.Typeflag)
		}

		inode++
	}

}

var rootfsTarPath = filepath.Join(outDir, "rootfs.tar")

func RootfsTar(_ context.Context, baseTar string) error {
	//todo: intermediary unpacking of base and delta saves ~4% (and prevents duplicates)
	mg.Deps(mg.F(mkdir, filepath.Dir(rootfsTarPath)), DeltaTarGz)
	build, err := target.Path(rootfsTarPath, baseTar, deltaTarGzPath)
	if !build {
		if mg.Verbose() {
			log.Printf(nopTargetFmt, rootfsTarPath)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("%v: %w", errMissingDep, err)
	}

	//todo: check if its gzip-ed or not
	base, baseCleanup, err := openTarFile(baseTar)
	if err != nil {
		return fmt.Errorf("open base rootfs file %q: %w", baseTar, err)
	}
	defer baseCleanup()

	delta, deltaCleanup, err := openTarGzFile(deltaTarGzPath)
	if err != nil {
		return fmt.Errorf("open delta tar file %q: %w", deltaTarGzPath, err)
	}
	defer deltaCleanup()

	rootfs, cleanup, err := newTarFile(rootfsTarPath)
	if err != nil {
		return fmt.Errorf("create rootfs tar file %q: %w", rootfsTarPath, err)
	}
	defer cleanup()

	if err := copyTarFiles(rootfs, base); err != nil {
		return fmt.Errorf("error reading tar file %q: %w", baseTar, err)
	}
	if err := copyTarFiles(rootfs, delta); err != nil {
		return fmt.Errorf("error reading tar file %q: %w", deltaTarGzPath, err)
	}

	//TODO: add these two
	// {dest: "./info/image.name", data: nil},
	// {dest: "./info/image.build.date", data: nil},

	if mg.Verbose() {
		log.Printf("created rootfs tar file %q\n", rootfsTarPath)
	}

	return nil
}

func copyTarFiles(dst *tar.Writer, src *tar.Reader) error {
	for {
		hdr, err := src.Next()
		switch err {
		case nil:
		case io.EOF:
			return nil
		default:
			return err
		}

		if err := dst.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := io.Copy(dst, src); err != nil {
			return err
		}
	}
}

var deltaTarGzPath = filepath.Join(outDir, "delta.tar.gz")

func DeltaTarGz(_ context.Context) error {
	//todo: add init and vsockexec as deps
	mg.Deps(mg.F(mkdir, filepath.Dir(deltaTarGzPath)), Build.GCS, Build.GCSTools, Build.WaitPaths)
	// since these are stamp files and not the sources themselves, ignore error if any are missing
	build, _ := target.Path(deltaTarGzPath,
		getFileStampKey(filepath.Join(cmdBin, "gcs")),
		getFileStampKey(filepath.Join(cmdBin, "gcstools")),
		getFileStampKey(filepath.Join(cmdBin, "wait-paths")),
	)
	if !build {
		if mg.Verbose() {
			log.Printf(nopTargetFmt, deltaTarGzPath)
		}
		return nil
	}

	now := time.Now()
	delta, cleanup, err := newTarGzFile(deltaTarGzPath)
	if err != nil {
		return fmt.Errorf("create delta tar file %q: %w", deltaTarGzPath, err)
	}
	defer cleanup()

	for _, d := range []string{"./", "./bin", "./info"} {
		if err := delta.WriteHeader(&tar.Header{
			Typeflag: tar.TypeDir,
			Name:     d,
			Mode:     0755,
			ModTime:  now,
			Format:   tar.FormatUSTAR,
		}); err != nil {
			return err
		}
	}

	files := []struct {
		source, dest string
	}{
		{source: filepath.Join(binDir, "init"), dest: "./init"},
		{source: filepath.Join(binDir, "vsockexec"), dest: "./bin/vsockexec"},
		{source: filepath.Join(cmdBin, "gcs"), dest: "./bin/gcs"},
		{source: filepath.Join(cmdBin, "gcstools"), dest: "./bin/gcstools"},
		{source: filepath.Join(cmdBin, "wait-paths"), dest: "./bin/wait-paths"},
	}
	for _, f := range files {
		file, err := os.Open(f.source)
		if err != nil {
			return err
		}
		defer file.Close()
		fi, err := file.Stat()
		if err != nil {
			return err
		}

		if err := delta.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     f.dest,
			Size:     fi.Size(),
			Mode:     0755,
			ModTime:  fi.ModTime(),
			Format:   tar.FormatUSTAR,
		}); err != nil {
			return fmt.Errorf("writing file %q header: %w", f.dest, err)
		}
		if _, err = io.Copy(delta, file); err != nil {
			return fmt.Errorf("writing file %q: %w", f.dest, err)
		}
	}

	links := []struct {
		link, dest string
	}{
		{link: "gcstools", dest: "./bin/generichook"},
		{link: "gcstools", dest: "./bin/install-drivers"},
	}
	for _, f := range links {
		if err := delta.WriteHeader(&tar.Header{
			Typeflag: tar.TypeSymlink,
			Name:     f.dest,
			Linkname: f.link,
			Mode:     0777,
			ModTime:  now,
			Format:   tar.FormatUSTAR,
		}); err != nil {
			return fmt.Errorf("writing link file %q header: %w", f.dest, err)
		}
	}

	// todo: embed this info directly into the GCS binary
	commit, err := sh.Output("git", "-C", rootDir, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("retrive git commit hash; %w", err)
	}
	branch, err := sh.Output("git", "-C", rootDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("retrive git branch name: %w", err)
	}

	info := []struct {
		dest string
		data []byte
	}{
		{dest: "./info/tar.date", data: []byte(now.UTC().Format("2022-09-21T01:07+00:00"))},
		{dest: "./info/gcs.commit", data: []byte(commit)},
		{dest: "./info/gcs.branch", data: []byte(branch)},
	}
	for _, f := range info {
		if f.data == nil {
			continue
		}

		if err := delta.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     f.dest,
			Size:     int64(len(f.data)),
			Mode:     0644,
			ModTime:  now,
			Format:   tar.FormatUSTAR,
		}); err != nil {
			return fmt.Errorf("writing info file %q header: %w", f.dest, err)
		}
		if _, err := delta.Write(f.data); err != nil {
			return fmt.Errorf("writing info file %q data %q: %w", f.dest, f.data, err)
		}
	}

	if mg.Verbose() {
		log.Printf("created delta tar file %q\n", deltaTarGzPath)
	}

	return nil
}

func openTarGzFile(path string) (*tar.Reader, func(), error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, nil, fmt.Errorf("gzip reader error: %w", err)
	}
	tr := tar.NewReader(gr)
	close := func() {
		_ = gr.Close()
		_ = f.Close()
	}
	return tr, close, nil
}

func openTarFile(path string) (*tar.Reader, func(), error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	tr := tar.NewReader(f)
	close := func() {
		_ = f.Close()
	}
	return tr, close, nil
}

func newTarGzFile(path string) (*tar.Writer, func(), error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	close := func() {
		_ = tw.Close()
		_ = gw.Close()
		_ = f.Close()
	}
	return tw, close, nil
}

func newTarFile(path string) (*tar.Writer, func(), error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	tw := tar.NewWriter(f)
	close := func() {
		_ = tw.Close()
		_ = f.Close()
	}
	return tw, close, nil
}

// GoGenerate (re)generates files created by `//go:generate` directives.
func GoGenerate(_ context.Context) error {
	//todo: find files with generate directives and compare timestamps with dummy file?
	//todo: cache list of files found?
	return sh.RunWith(varMap{"GOWORK": "off", "GOOS": "windows"}, goCmd(), "generate", "-x", rootDir+"/...")
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

// Validate checks calls mod tidy (and vendor) and lints the repo.
func Validate(ctx context.Context) {
	mg.SerialCtxDeps(ctx, ModRepo, Lint.Repo)
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
		"cmd/gcs", "cmd/gcstools", "internal/guest", "internal/tools", "pkg")
}

func (Lint) TestLinux(ctx context.Context) error {
	return lint(ctx, testDir, "linux")
}

func lint(ctx context.Context, dir, goos string, paths ...string) error {
	// todo: check if linter exists, and install otherwise
	mg.Deps(mg.F(mkdir, dir))

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

// Clean removes (test) executables, and other output artifacts.
func Clean(_ context.Context) error {
	for _, dir := range []string{binDir, outDir, depsDir, protobufDir} {
		if err := sh.Rm(dir); err != nil {
			return err
		}
	}
	return nil
}

//
// Helpers
//

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

func mkdir(p string) error {
	if err := os.MkdirAll(p, 0750); err != nil {
		return fmt.Errorf("creating %q: %w", p, err)
	}
	return nil
}

// adds the path p to the PATH environment variable
func addToPath(p string) string {
	return p + string(os.PathListSeparator) + os.Getenv("PATH")
}
