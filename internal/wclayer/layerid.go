//go:build windows

package wclayer

import (
	"context"
	"path/filepath"

	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/Microsoft/hcsshim/internal/otel"
	"go.opentelemetry.io/otel/attribute"
)

// LayerID returns the layer ID of a layer on disk.
func LayerID(ctx context.Context, path string) (_ guid.GUID, err error) {
	title := "hcsshim::LayerID"
	ctx, span := otel.StartSpan(ctx, title)
	defer func() { otel.SetSpanStatusAndEnd(span, err) }()
	span.SetAttributes(attribute.String("path", path))

	_, file := filepath.Split(path)
	return NameToGuid(ctx, file)
}
