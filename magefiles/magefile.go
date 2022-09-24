//go:build mage

package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Microsoft/hcsshim/ext4/tar2ext4"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/magefile/mage/target"
)

// Match `date --iso-8601=minute --utc` on Linux
const ISO8601Minute = "2006-01-02T15:04-0700"

const nopTargetFmt = "no work to do for %q\n"

var errMissingDep = errors.New("missing dependency")

var (
	// rootDir the full path to the repo root.
	//
	// mage runs with magefiles as the working directory, but that can be changed with the
	// `-w` flag, and the compiled binary can be put anywhere.
	// So this is used to locate paths relative to the root of the repo.
	rootDir = getRootDir()

	binDir  = filepath.Join(rootDir, "bin")
	cmdBin  = filepath.Join(binDir, "cmd")  // binaries from ./cmd and ./internal/tools
	testBin = filepath.Join(binDir, "test") // binaries from ./test
	toolBin = filepath.Join(binDir, "tool") // dependencies (ie, protobuf)

	outDir      = filepath.Join(rootDir, "out")      // intermediary products and outputs
	depsDir     = filepath.Join(rootDir, "deps")     // dependency tracking
	protobufDir = filepath.Join(rootDir, "protobuf") // protobuf includes (see Protobuild.toml)

	testDir = filepath.Join(rootDir, "test")
)

// var testFlag = flag.Bool("test", false, "test flag")
// var trueArgs = os.Args

func init() {
	// // todo: strip out custom flags from os.Args to be none-the-wiser
	// fmt.Println(os.Args)
	// fmt.Println("true args", trueArgs)
	// flag.Parse()
	// fmt.Println("the test flag is:", *testFlag)

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
	"lint":   Lint.Repo,
	"delta":  DeltaTarGz,
	"rootfs": RootfsVHD,
	"initrd": Initramfs,

	// default build target is the shim
	"build": Build.Shim,
	"shim":  Build.Shim,

	// default BuildTest target is All
	"buildtest": BuildTest.All,
	"critest":   BuildTest.CRIContainerd,
	"functest":  BuildTest.Functional,
}

// Util houses helper targets that are not generally needed
type Util mg.Namespace

// List is for debugging the mage binary
func (Util) List(_ context.Context) {}

//todo: protobuf
//todo: call wsl for init and vsock
//todo: rego support to gcs, shim, and test exes
//todo: pull/build docker images for cri-containerd\test-images
//todo: run unit tests
//todo: mg.Verbose-aware logger
//todo: dependencies (golangci-lint, docker, protoc)
//todo: find mimimal set of env variables needed to run go and co

//
// general file and artifact generation (go gen and protoc)
//

var rootfsVHDPath = filepath.Join(outDir, "rootfs.vhd")

func RootfsVHD(_ context.Context, baseTar string) error {
	mkdir(filepath.Dir(rootfsVHDPath))
	mg.Deps(mg.F(RootfsTar, baseTar))
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
	mkdir(filepath.Dir(initramfsPath))
	mg.Deps(mg.F(RootfsTar, baseTar))
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

	if err := convertTarToInitramfs(initramfsPath, rootfsTarPath); err != nil {
		return fmt.Errorf("converting rootfs tar %q to initramfs %q: %w",
			rootfsTarPath, initramfsPath, err)
	}
	if mg.Verbose() {
		log.Printf("created initramfs file %q\n", initramfsPath)
	}
	return nil
}

var rootfsTarPath = filepath.Join(outDir, "rootfs.tar")

func RootfsTar(ctx context.Context, baseTar string) error {
	mkdir(filepath.Dir(rootfsTarPath))
	mg.Deps(DeltaTarGz)
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

	return (Util{}).RootfsTar(ctx, baseTar)
}

// RootfsTar is like [RootfsTar], but does not check dependency timestamps.
// This is helpful in CI pipelines where the delta.tar and base rootfs are built elsewhere,
// and they only need to be combined.
func (Util) RootfsTar(_ context.Context, baseTar string) error {
	//TODO: add these two
	// {dest: "./info/image.name", data: nil},
	// {dest: "./info/image.build.date", data: nil},

	if err := mergeTarFiles(rootfsTarPath, baseTar, deltaTarGzPath); err != nil {
		return fmt.Errorf("error creating rootfs tar file %q: %w", rootfsTarPath, err)
	}

	if mg.Verbose() {
		log.Printf("created rootfs tar file %q\n", rootfsTarPath)
	}

	return nil
}

var deltaTarGzPath = filepath.Join(outDir, "delta.tar.gz")

func DeltaTarGz(_ context.Context) error {
	mkdir(filepath.Dir(deltaTarGzPath))
	//todo: add init and vsockexec as deps
	mg.Deps(Build.GCS, Build.GCSTools, Build.WaitPaths)
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

	// do this manually, so we can control permissions and uid/gid

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
		{dest: "./info/tar.date", data: []byte(now.UTC().Format(ISO8601Minute))},
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
	if _, err := Exec(ctx, goCmd(),
		[]string{"mod", "tidy", "-e", "-v"},
		execInDir(rootDir),
		execInheritEnv,
		execVerbose,
	); err != nil {
		return err
	}
	_, err := Exec(ctx, goCmd(),
		[]string{"mod", "vendor", "-e"},
		execInDir(rootDir),
		execInheritEnv,
		execVerbose)
	return err
}

// ModTest runs `go mod tidy` on `./test`.
func ModTest(ctx context.Context) error {
	_, err := Exec(ctx, goCmd(),
		[]string{"mod", "tidy", "-e", "-v"},
		execInDir(testDir),
		execInheritEnv,
		execVerbose)
	return err
}

//
// misc and cleanup
//

// Validate checks calls mod tidy (and vendor) and lints the repo.
func Validate(ctx context.Context) {
	mg.SerialCtxDeps(ctx, ModRepo, Lint.Repo)
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

// Rebuild (re)creates the mage executable in the root of the repo.
func (Util) Self(ctx context.Context) error {
	// output is relative to the magefile directory
	args := []string{"run", "./magefiles/mage.go", "-f", "-d", "./magefiles", "-compile", "../build.exe"}
	if mg.Verbose() {
		args = append(args, "-debug")
	}
	_, err := Exec(ctx, goCmd(),
		args,
		execInDir(rootDir),
		execInheritEnv,
		execWithEnv(varMap{
			"MAGEFILE_HASHFAST":     "false",
			"MAGEFILE_ENABLE_COLOR": "true",
		}),
		execVerbose,
	)
	return err
}

//
// Helpers
//

// adds the path p to the PATH environment variable
func addToPath(p string) string {
	return p + string(os.PathListSeparator) + os.Getenv("PATH")
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

// convenience function: rather than returning an error and write out
// `if err:= ...; err != nil { return error }`, just panic
func mkdir(p string) {
	if err := os.MkdirAll(p, 0750); err != nil {
		panic(fmt.Sprintf("could not create %q: %v", p, err))
	}
}

// getRootDir returns the absolute path to the root of the repo.
func getRootDir() string {
	// Can also find root with `go list -f '{{.Root}}' .`, but that assumes working directory
	// is a valid go pkg.
	// Embedding a dummy file as a embed.FS and accessing the fs.FileInfo would also work.
	// "runtime/debug".BuildInfo doesn't have a valid path if the binary is not compiled.
	_, r, _, ok := runtime.Caller(0)
	if !ok {
		panic("could not find root path for module")
	}
	return filepath.Dir(filepath.Dir(r))
}
