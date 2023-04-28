// helper functions for dealing with OTel
package otel

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
)

var (
	// ErrInvalidSpanContext indicates the span did not contain valid trace or span ID
	ErrInvalidSpanContext = errors.New("invalid span context")
	// ErrSpanExport indicates an error after serializing the span, during the export stage.
	ErrSpanExport = errors.New("span export")
	// ErrNoETWProvider is returned when there is no configured ETW provider.
	ErrNoETWProvider = errors.New("no ETW provider")
)

func init() {
	// register TraceContext propagator to pass trace conext and baggage (if any)  over GCS
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
}

func InjectContext(ctx context.Context, carrier propagation.TextMapCarrier) {
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}

func ExtractContext(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

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
