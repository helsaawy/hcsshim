// The package contains setup and other components fir OpenTelemetry support.
package otel

import (
	"errors"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
)

// InstrumentationName is the name of the OpenTelemetry [metric.Meter] or
// [trace.Tracer] used to instrument this repo.
//
// Use one instrumentation library provider to simplify code.
const InstrumentationName = "github.com/Microsoft/hcsshim"

// ErrNoETWProvider is returned when there is no configured ETW provider.
var ErrNoETWProvider = errors.New("no ETW provider")

// TODO: add azure host resource detector
// https://github.com/open-telemetry/opentelemetry-go-contrib/tree/main/detectors

func DefaultResource(appName, appVersion string, attrs ...attribute.KeyValue) *resource.Resource {
	// required fields
	// https://opentelemetry.io/docs/specs/otel/resource/semantic_conventions/#telemetry-sdk
	as := []attribute.KeyValue{
		semconv.TelemetrySDKLanguageGo,
		semconv.TelemetrySDKName("go.opentelemetry.io/otel"),
		semconv.TelemetrySDKVersion(otel.Version()),
	}

	// recommended fields
	// https://opentelemetry.io/docs/specs/otel/resource/semantic_conventions/#service-experimental
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
