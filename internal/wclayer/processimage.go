//go:build windows

package wclayer

import (
	"context"
	"os"

	"github.com/Microsoft/hcsshim/internal/otel"
	"go.opentelemetry.io/otel/attribute"
)

// ProcessBaseLayer post-processes a base layer that has had its files extracted.
// The files should have been extracted to <path>\Files.
func ProcessBaseLayer(ctx context.Context, path string) (err error) {
	title := "hcsshim::ProcessBaseLayer"
	ctx, span := otel.StartSpan(ctx, title) //nolint:ineffassign,staticcheck
	defer func() { otel.SetSpanStatusAndEnd(span, err) }()
	span.SetAttributes(attribute.String("path", path))

	err = processBaseImage(path)
	if err != nil {
		return &os.PathError{Op: title, Path: path, Err: err}
	}
	return nil
}

// ProcessUtilityVMImage post-processes a utility VM image that has had its files extracted.
// The files should have been extracted to <path>\Files.
func ProcessUtilityVMImage(ctx context.Context, path string) (err error) {
	title := "hcsshim::ProcessUtilityVMImage"
	ctx, span := otel.StartSpan(ctx, title) //nolint:ineffassign,staticcheck
	defer func() { otel.SetSpanStatusAndEnd(span, err) }()
	span.SetAttributes(attribute.String("path", path))

	err = processUtilityImage(path)
	if err != nil {
		return &os.PathError{Op: title, Path: path, Err: err}
	}
	return nil
}
