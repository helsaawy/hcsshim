// The package contains setup and other components fir OpenTelemetry support.
package otel

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// InstrumentationName is the name of the OpenTelemetry [metric.Meter] or
// [trace.Tracer] used to instrument this repo.
//
// Use one instrumentation library provider to simplify code.
const InstrumentationName = "github.com/Microsoft/hcsshim"

// TODO: add azure host resource detector
// https://github.com/open-telemetry/opentelemetry-go-contrib/tree/main/detectors

// DefaultResource creates a [resource.Resource] for use in shim code with the specified service
// name and version (if valid).
//
// Regardless of the error, following OTel specifications, the returned [resource.Resource]
// is always valid.
func DefaultResource(ctx context.Context, svcName, svcVersion string, attrs ...attribute.KeyValue) (*resource.Resource, error) {
	// first create a minimal resource with the service info and other identifying fields, so if
	// the detectors or merge fail, the fall back resource still has enough info to be useful

	// additionally, we need to specify the SchemaURL for our attributes, which is only possible
	// if we create the resource manually

	// service fields
	if svcName != "" {
		attrs = append(attrs, semconv.ServiceName(svcName))
	}
	if svcVersion != "" {
		attrs = append(attrs, semconv.ServiceVersion(svcVersion))
	}

	var rsc *resource.Resource
	if len(attrs) > 0 {
		rsc = resource.NewWithAttributes(semconv.SchemaURL, attrs...)
	}
	rsc2, err := resource.New(ctx,
		resource.WithHost(),
		resource.WithOSDescription(),
		resource.WithTelemetrySDK(),
		resource.WithFromEnv(),
	)
	if err != nil {
		err = fmt.Errorf("detecting resource attributes: %w", err)
	}

	// try to merge the two resources
	// rsc2 is still valid, and may contain some attributes, so merge regardless of err
	// rsc takes precedence and will overwrite shared values of rsc2
	mRsc, mErr := resource.Merge(rsc2, rsc)
	if !mRsc.Equal(resource.Empty()) || mErr != nil {
		// check if the merge failed, so fall back to our resource
		rsc = mRsc
	}
	if mErr != nil {
		mErr = fmt.Errorf("merging resources: %w", mErr)
		if err != nil {
			// TODO (go1.20): use multierror via fmt.Errorf
			err = fmt.Errorf("%v; %v", err, mErr)
		} else {
			err = mErr
		}
	}

	return rsc, err
}
