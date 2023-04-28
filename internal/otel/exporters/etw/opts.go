//go:build windows

package etw

import "github.com/Microsoft/go-winio/pkg/etw"

type Option func(*exporter) error

// WithNewETWProvider registers a new ETW provider for the exporter to use.
// The provider will be closed when the exporter is shutdown.
func WithNewETWProvider(n string) Option {
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

// WithExistingETWProvider configures the exporter to use an existing ETW provider.
// The provider will not be closed when the exporter is shutdown.
func WithExistingETWProvider(p *etw.Provider) Option {
	return func(e *exporter) error {
		e.provider = p
		e.closeProvider = false
		return nil
	}
}

// SetActivityID specifies if the ETW events should have their (related) activity ID
// set to the (parent) span ID.
//
// This is useful for correlating spans with other ETW events.
func SetActivityID(b bool) Option {
	return func(e *exporter) error {
		e.setActivityID = b
		return nil
	}
}

// WithSpanETWLevel specifies the [etw.Level] to use when exporting spans to ETW events.
//
// The default is [etw.LevelInfo].
func WithSpanETWLevel(l etw.Level) Option {
	return func(e *exporter) error {
		e.level = l
		return nil
	}
}

// WithErrorSpanETWLevel specifies the [etw.Level] to use when exporting spans whose status code
// is [go.opentelemetry.io/otel/codes.Error].
//
// The default is [etw.LevelError].
func WithErrorSpanETWLevel(l etw.Level) Option {
	return func(e *exporter) error {
		e.level = l
		return nil
	}
}
