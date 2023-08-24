// helper functions for dealing with OTel
package otel

import (
	"errors"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
)

// InstrumentationName is the name of OTel ["go.opentelemetry.io/otel/metric".Meter] or
// ["go.opentelemetry.io/otel/trace".Tracer] used in this repo.
//
// Use one instrumentation library provider to simplify code.
const InstrumentationName = "github.com/Microsoft/hcsshim"

// ErrNoETWProvider is returned when there is no configured ETW provider.
var ErrNoETWProvider = errors.New("no ETW provider")

func DefaultResource(appName, appVersion string, attrs ...attribute.KeyValue) *resource.Resource {
	as := []attribute.KeyValue{
		semconv.TelemetrySDKLanguageGo,
		semconv.TelemetrySDKName("opentelemetry"),
		semconv.TelemetrySDKVersion(otel.Version()),
	}

	if appName != "" {
		as = append(as, semconv.ServiceName(appName))
	}
	if appVersion != "" {
		as = append(as, semconv.ServiceVersion(appVersion))
	}
	as = append(as, attrs...)
	return resource.NewWithAttributes(semconv.SchemaURL, as...)
}

// Create an appropriate telemetry name (ie, for metrics and spans) from the components.
//
// Component names are joined via "::".
func Name(names ...string) string {
	ns := make([]string, 0, len(names))
	// filter out empty strings
	for _, n := range names {
		if n != "" {
			ns = append(ns, n)
		}
	}
	return strings.Join(ns, "::")
}
