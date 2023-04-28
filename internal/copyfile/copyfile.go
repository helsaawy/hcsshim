//go:build windows

package copyfile

import (
	"context"
	"fmt"
	"syscall"

	"github.com/Microsoft/hcsshim/internal/otel"
	"github.com/Microsoft/hcsshim/internal/winapi"
	"go.opentelemetry.io/otel/attribute"
)

// CopyFile is a utility for copying a file using CopyFileW win32 API for
// performance.
func CopyFile(ctx context.Context, srcFile, destFile string, overwrite bool) (err error) {
	ctx, span := otel.StartSpan(ctx, "copyfile::CopyFile") //nolint:ineffassign,staticcheck
	defer func() { otel.SetSpanStatusAndEnd(span, err) }()
	span.SetAttributes(
		attribute.String("srcFile", srcFile),
		attribute.String("destFile", destFile),
		attribute.Bool("overwrite", overwrite))

	var bFailIfExists int32 = 1
	if overwrite {
		bFailIfExists = 0
	}

	lpExistingFileName, err := syscall.UTF16PtrFromString(srcFile)
	if err != nil {
		return err
	}
	lpNewFileName, err := syscall.UTF16PtrFromString(destFile)
	if err != nil {
		return err
	}
	if err := winapi.CopyFileW(lpExistingFileName, lpNewFileName, bFailIfExists); err != nil {
		return fmt.Errorf("failed CopyFileW Win32 call from '%s' to '%s': %s", srcFile, destFile, err)
	}
	return nil
}
