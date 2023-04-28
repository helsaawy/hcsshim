//go:build windows

package computestorage

import (
	"context"

	"github.com/Microsoft/hcsshim/internal/otel"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
)

// DetachLayerStorageFilter detaches the layer storage filter on a writable container layer.
//
// `layerPath` is a path to a directory containing the layer to export.
func DetachLayerStorageFilter(ctx context.Context, layerPath string) (err error) {
	title := "hcsshim::DetachLayerStorageFilter"
	ctx, span := otel.StartSpan(ctx, title) //nolint:ineffassign,staticcheck
	defer func() { otel.SetSpanStatusAndEnd(span, err) }()
	span.SetAttributes(attribute.String("layerPath", layerPath))

	err = hcsDetachLayerStorageFilter(layerPath)
	if err != nil {
		return errors.Wrap(err, "failed to detach layer storage filter")
	}
	return nil
}
