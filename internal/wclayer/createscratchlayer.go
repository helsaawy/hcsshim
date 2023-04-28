//go:build windows

package wclayer

import (
	"context"
	"strings"

	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/Microsoft/hcsshim/internal/otel"
	"go.opentelemetry.io/otel/attribute"
)

// CreateScratchLayer creates and populates new read-write layer for use by a container.
// This requires the full list of paths to all parent layers up to the base
func CreateScratchLayer(ctx context.Context, path string, parentLayerPaths []string) (err error) {
	title := "hcsshim::CreateScratchLayer"
	ctx, span := otel.StartSpan(ctx, title)
	defer func() { otel.SetSpanStatusAndEnd(span, err) }()
	span.SetAttributes(
		attribute.String("path", path),
		attribute.String("parentLayerPaths", strings.Join(parentLayerPaths, ", ")))

	// Generate layer descriptors
	layers, err := layerPathsToDescriptors(ctx, parentLayerPaths)
	if err != nil {
		return err
	}

	err = createSandboxLayer(&stdDriverInfo, path, 0, layers)
	if err != nil {
		return hcserror.New(err, title, "")
	}
	return nil
}
