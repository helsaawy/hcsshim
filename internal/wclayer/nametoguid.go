//go:build windows

package wclayer

import (
	"context"

	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/Microsoft/hcsshim/internal/otel"
	"go.opentelemetry.io/otel/attribute"
)

// NameToGuid converts the given string into a GUID using the algorithm in the
// Host Compute Service, ensuring GUIDs generated with the same string are common
// across all clients.
func NameToGuid(ctx context.Context, name string) (_ guid.GUID, err error) {
	title := "hcsshim::NameToGuid"
	ctx, span := otel.StartSpan(ctx, title) //nolint:ineffassign,staticcheck
	defer func() { otel.SetSpanStatusAndEnd(span, err) }()
	span.SetAttributes(attribute.String("objectName", name))

	var id guid.GUID
	err = nameToGuid(name, &id)
	if err != nil {
		return guid.GUID{}, hcserror.New(err, title, "")
	}
	span.SetAttributes(attribute.String("guid", id.String()))
	return id, nil
}
