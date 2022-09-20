//go:build mage

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

type execOpt func(*exec.Cmd) error

func execInDir(dir string) execOpt {
	return func(c *exec.Cmd) error {
		c.Dir = dir
		return nil
	}
}

func execWithEnv(env map[string]string) execOpt {
	e := flattenEnv(env)
	return func(c *exec.Cmd) error {
		c.Env = append(c.Env, e...)
		return nil
	}
}

func execInheritEnv() execOpt {
	return func(c *exec.Cmd) error {
		c.Env = append(c.Env, os.Environ()...)
		return nil
	}
}

func flattenEnv(env map[string]string) []string {
	e := make([]string, 0, len(env))
	for k, v := range env {
		e = append(e, k+"="+v)
	}
	return e
}

//nolint:unused
func execWithStdIO(stdIn io.Reader, stdOut, stdErr io.Writer) execOpt {
	return func(c *exec.Cmd) error {
		c.Stdin = stdIn
		c.Stdout = stdOut
		c.Stderr = stdErr
		return nil
	}
}

func execVerbose(c *exec.Cmd) error {
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return nil
}

// Exec is a custom implementation of [sh.Exec], until they add working directory support.
func Exec(ctx context.Context, cmd string, args []string, opts ...execOpt) (bool, error) {
	c := exec.CommandContext(ctx, cmd, args...)
	for _, o := range opts {
		if err := o(c); err != nil {
			return false, err
		}
	}

	if mg.Verbose() {
		quoted := make([]string, 0, len(args))
		for _, a := range args {
			quoted = append(quoted, fmt.Sprintf("%q", a))
		}
		log.Println("exec:", cmd, strings.Join(quoted, " "))
	}

	err := c.Run()
	ran := sh.CmdRan(err)
	code := sh.ExitStatus(err)
	if err == nil {
		return true, nil
	}
	if ran {
		return ran, mg.Fatalf(code, `running "%s" failed with exit code %d`, c.String(), code)
	}
	return ran, fmt.Errorf(`failed to run "%s: %v"`, c.String(), err)
}
