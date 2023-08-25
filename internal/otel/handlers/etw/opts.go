//go:build windows

package etw

import (
	"github.com/Microsoft/go-winio/pkg/etw"
	"go.opentelemetry.io/otel/sdk/resource"

	oteletw "github.com/Microsoft/hcsshim/internal/otel/etw"
)

type Option func(*handler) error

// WithProvider configures the exporter to use an existing ETW provider.
func WithProvider(p *etw.Provider) Option {
	return func(h *handler) error {
		h.provider = p
		return nil
	}
}

// WithResource specifies an OTel [resource.Resource] to append to the error message.
func WithResource(rsc *resource.Resource) Option {
	f := oteletw.SerializeResource(rsc)
	return func(h *handler) error {
		h.extra = append(h.extra, f)
		return nil
	}
}

// WithExtra specifies additional [etw.FieldOpts] to append to the error message.
func WithExtra(fields ...etw.FieldOpt) Option {
	return func(h *handler) error {
		h.extra = append(h.extra, fields...)
		return nil
	}
}

// WithLevel specifies the [etw.Level] to use when writing errors as ETW events.
//
// The default is [etw.LevelError].
func WithLevel(l etw.Level) Option {
	return func(h *handler) error {
		h.level = l
		return nil
	}
}
