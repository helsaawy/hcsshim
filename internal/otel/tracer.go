package otel

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"go.opentelemetry.io/otel/trace"
)

//todo (helsaawy) propagator/extractor for HCS protocol requests

// InitializeProvider sets the global OTel TraceProvider.
//
// If no exporter is provided, no spans will be generated.
func InitializeProvider(opts ...tracesdk.TracerProviderOption) (func(context.Context) error, error) {
	tracerProvider := tracesdk.NewTracerProvider(opts...)
	otel.SetTracerProvider(tracerProvider)

	f := func(ctx context.Context) error {
		err := tracerProvider.ForceFlush(ctx)
		// shutdown regardless of flush result
		if err2 := tracerProvider.Shutdown(ctx); err == nil && err2 != nil {
			return err2
		}
		return err
	}
	return f, nil
}

func SetTraceContextPropagation() {
	otel.SetTextMapPropagator(propagation.TraceContext{})
}

func tracer() trace.Tracer {
	// for now, one instrumentation for the entire repo
	return otel.Tracer(
		// use dedicated Tracer in case imported code modifies the global default.
		"github.com/Microsoft/hcsshim",
		trace.WithSchemaURL(semconv.SchemaURL),
	)
}
