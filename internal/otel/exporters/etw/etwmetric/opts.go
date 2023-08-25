//go:build windows

package etwmetric

import (
	"github.com/Microsoft/go-winio/pkg/etw"
	"go.opentelemetry.io/otel/sdk/metric"
)

type Option func(*exporter) error

// WithTemporalitySelector sets the TemporalitySelector the exporter will use
// to determine the Temporality of an instrument based on its kind.
// If this option is not used, the exporter will use
// ["go.opentelemetry.io/otel/sdk/metric".DefaultTemporalitySelector].
func WithTemporalitySelector(selector metric.TemporalitySelector) Option {
	return func(e *exporter) error {
		e.temporality = selector
		return nil
	}
}

// WithAggregationSelector sets the AggregationSelector the exporter will use
// to determine the aggregation to use for an instrument based on its kind. If
// this option is not used, the exporter will use
// ["go.opentelemetry.io/otel/sdk/metric".DefaultAggregationSelector]
// or the aggregation explicitly passed for a view matching an instrument.
func WithAggregationSelector(selector metric.AggregationSelector) Option {
	return func(e *exporter) error {
		e.aggregation = selector
		return nil
	}
}

// WithNewProvider registers a new ETW provider for the exporter to use.
// The provider will be closed when the exporter is shutdown.
func WithNewProvider(n string) Option {
	return func(e *exporter) error {
		provider, err := etw.NewProvider(n, nil)
		if err != nil {
			return err
		}

		e.provider = provider
		e.closeProvider = true
		return nil
	}
}

// WithExistingProvider configures the exporter to use an existing ETW provider.
// The provider will not be closed when the exporter is shutdown.
func WithExistingProvider(p *etw.Provider) Option {
	return func(e *exporter) error {
		e.provider = p
		e.closeProvider = false
		return nil
	}
}

// WithLevel specifies the [etw.Level] to use when exporting metrics to ETW events.
//
// The default is [etw.LevelInfo].
func WithLevel(l etw.Level) Option {
	return func(e *exporter) error {
		e.level = l
		return nil
	}
}
