//go:build windows

package etw

import (
	"errors"

	"github.com/Microsoft/go-winio/pkg/etw"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
)

const FieldResource = "otel.resource"

// ErrNoETWProvider is returned when there is no configured ETW provider.
var ErrNoETWProvider = errors.New("no ETW provider")

// SerializeResource returns an [etw.Struct] field option containing the [resource.Resource]
// attributes.
func SerializeResource(rsc *resource.Resource) etw.FieldOpt {
	if fs := SerializeAttributes(rsc.Attributes()); len(fs) > 0 {
		return etw.Struct(FieldResource, fs...)
	}
	return nil
}

func SerializeAttributes(attrs []attribute.KeyValue) []etw.FieldOpt {
	fields := make([]etw.FieldOpt, 0, len(attrs))

	for _, attr := range attrs {
		// AsInterface() will convert to the right field type based on OTel's supported field types,
		// and then etw.SmartField will do its own type-matching.
		fields = append(fields, etw.SmartField(string(attr.Key), attr.Value.AsInterface()))
	}
	return fields
}
