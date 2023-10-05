//go:build windows

package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/Microsoft/hcsshim/internal/cmd"
)

type BufferedIO struct {
	in       *bytes.Buffer
	out, err bytes.Buffer
}

func NewBufferedIOFromString(in string) *BufferedIO {
	b := NewBufferedIO()
	b.in = bytes.NewBufferString(in)

	return b
}

func NewBufferedIO() *BufferedIO {
	return &BufferedIO{}
}

func (b *BufferedIO) Output() (_ string, err error) {
	o := b.out.String()
	if e := b.err.String(); len(e) != 0 {
		err = errors.New(e)
	}

	return o, err
}

func (b *BufferedIO) TestOutput(tb testing.TB, out string, err error, trim bool) {
	tb.Helper()

	outGot, errGot := b.Output()
	if !errors.Is(errGot, err) {
		tb.Fatalf("got stderr: %v; wanted: %v", errGot, err)
	}
	if trim {
		out = strings.TrimSpace(out)
		outGot = strings.TrimSpace(outGot)
	}
	out = strings.ToLower(out)
	outGot = strings.ToLower(outGot)
	if diff := cmp.Diff(out, outGot); diff != "" {
		tb.Fatalf("stdout mismatch (-want +got):\n%s", diff)
	}
}

func (b *BufferedIO) AddToCmd(c *cmd.Cmd) {
	if b == nil {
		return
	}

	if c.Stdin == nil && b.in != nil {
		c.Stdin = b.in
	}
	if c.Stdout == nil {
		c.Stdout = &b.out
	}
	if c.Stderr == nil {
		c.Stderr = &b.err
	}
}
