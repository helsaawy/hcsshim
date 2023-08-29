// This package provider OTel metrics support for the shim
package metric

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	api "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	hcsotel "github.com/Microsoft/hcsshim/internal/otel"
)

// InitializeProvider sets the global OTel MeterProvider.
//
// If no reader is provided, returned Instruments will nop.
func InitializeProvider(opts ...metric.Option) (func(context.Context) error, error) {
	provider := metric.NewMeterProvider(opts...)
	// set it to the global meter provider
	otel.SetMeterProvider(provider)

	f := func(ctx context.Context) error {
		err := provider.ForceFlush(ctx)
		// TODO (go1.20): use multierrors to return both
		// shutdown regardless of flush result
		if err2 := provider.Shutdown(ctx); err == nil && err2 != nil {
			return err2
		}
		return err
	}
	return f, nil
}

func Meter(opts ...api.MeterOption) api.Meter {
	// TODO: if repo gets a global version value, add that here via [api.WithInstrumentationVersion]

	return otel.Meter(
		hcsotel.InstrumentationName,
		// append opts to default MeterOptions so they can take precedence and override defaults
		append([]api.MeterOption{api.WithSchemaURL(semconv.SchemaURL)}, opts...)...,
	)
}

// re-implement [api.Meter] functions, but instead of returning errors, use the OTel error handler.
//
// OTel specification (and SDK implementation) return a valid instrument, regardless of error status:
// https://opentelemetry.io/docs/specs/otel/library-guidelines/#api-and-minimal-implementation

// Int64Counter returns a Counter used to record int64 measurements.
func Int64Counter(name string, options ...api.Int64CounterOption) api.Int64Counter {
	i, err := Meter().Int64Counter(name, options...)
	if err != nil {
		onError(name, err)
	}
	return i
}

// Int64UpDownCounter returns an UpDownCounter used to record int64
// measurements.
func Int64UpDownCounter(name string, options ...api.Int64UpDownCounterOption) api.Int64UpDownCounter {
	i, err := Meter().Int64UpDownCounter(name, options...)
	if err != nil {
		onError(name, err)
	}
	return i
}

// Int64Histogram returns a Histogram used to record int64 measurements.
func Int64Histogram(name string, options ...api.Int64HistogramOption) api.Int64Histogram {
	i, err := Meter().Int64Histogram(name, options...)
	if err != nil {
		onError(name, err)
	}
	return i
}

// Int64ObservableCounter returns an ObservableCounter used to record int64
// measurements.
func Int64ObservableCounter(name string, options ...api.Int64ObservableCounterOption) api.Int64ObservableCounter {
	i, err := Meter().Int64ObservableCounter(name, options...)
	if err != nil {
		onError(name, err)
	}
	return i
}

// Int64ObservableUpDownCounter returns an ObservableUpDownCounter used to
// record int64 measurements.
func Int64ObservableUpDownCounter(name string, options ...api.Int64ObservableUpDownCounterOption) api.Int64ObservableUpDownCounter {
	i, err := Meter().Int64ObservableUpDownCounter(name, options...)
	if err != nil {
		onError(name, err)
	}
	return i
}

// Int64ObservableGauge returns an ObservableGauge used to record int64
// measurements.
func Int64ObservableGauge(name string, options ...api.Int64ObservableGaugeOption) api.Int64ObservableGauge {
	i, err := Meter().Int64ObservableGauge(name, options...)
	if err != nil {
		onError(name, err)
	}
	return i
}

// Float64Counter returns a Counter used to record int64 measurements that
// produces no telemetry.
func Float64Counter(name string, options ...api.Float64CounterOption) api.Float64Counter {
	i, err := Meter().Float64Counter(name, options...)
	if err != nil {
		onError(name, err)
	}
	return i
}

// Float64UpDownCounter returns an UpDownCounter used to record int64
// measurements.
func Float64UpDownCounter(name string, options ...api.Float64UpDownCounterOption) api.Float64UpDownCounter {
	i, err := Meter().Float64UpDownCounter(name, options...)
	if err != nil {
		onError(name, err)
	}
	return i
}

// Float64Histogram returns a Histogram used to record int64 measurements that
// produces no telemetry.
func Float64Histogram(name string, options ...api.Float64HistogramOption) api.Float64Histogram {
	i, err := Meter().Float64Histogram(name, options...)
	if err != nil {
		onError(name, err)
	}
	return i
}

// Float64ObservableCounter returns an ObservableCounter used to record int64
// measurements.
func Float64ObservableCounter(name string, options ...api.Float64ObservableCounterOption) api.Float64ObservableCounter {
	i, err := Meter().Float64ObservableCounter(name, options...)
	if err != nil {
		onError(name, err)
	}
	return i
}

// Float64ObservableUpDownCounter returns an ObservableUpDownCounter used to
// record int64 measurements.
func Float64ObservableUpDownCounter(name string, options ...api.Float64ObservableUpDownCounterOption) api.Float64ObservableUpDownCounter {
	i, err := Meter().Float64ObservableUpDownCounter(name, options...)
	if err != nil {
		onError(name, err)
	}
	return i
}

// Float64ObservableGauge returns an ObservableGauge used to record int64
// measurements.
func Float64ObservableGauge(name string, options ...api.Float64ObservableGaugeOption) api.Float64ObservableGauge {
	i, err := Meter().Float64ObservableGauge(name, options...)
	if err != nil {
		onError(name, err)
	}
	return i
}

// onError handles errors in the below instrument creation functions, since it is ignored and
// instead a nop version is returned.
func onError(name string, err error) {
	otel.Handle(fmt.Errorf("unable to create instrument %q from meter %q: %v", name, hcsotel.InstrumentationName, err))
}
