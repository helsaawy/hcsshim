//go:build windows

package wclayer

import (
	"context"

	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/Microsoft/hcsshim/internal/otel"
	"go.opentelemetry.io/otel/attribute"
)

// CreateLayer creates a new, empty, read-only layer on the filesystem based on
// the parent layer provided.
func CreateLayer(ctx context.Context, path, parent string) (err error) {
	title := "hcsshim::CreateLayer"
	ctx, span := otel.StartSpan(ctx, title) //nolint:ineffassign,staticcheck
	defer func() { otel.SetSpanStatusAndEnd(span, err) }()
	span.SetAttributes(
		attribute.String("path", path),
		attribute.String("parent", parent))

	err = createLayer(&stdDriverInfo, path, parent)
	if err != nil {
		return hcserror.New(err, title, "")
	}
	return nil
}
