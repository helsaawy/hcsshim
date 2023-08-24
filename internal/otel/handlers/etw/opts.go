//go:build windows

package etw

import "github.com/Microsoft/go-winio/pkg/etw"

type Option func(*handler) error

// WithExistingETWProvider configures the exporter to use an existing ETW provider.
// The provider will not be closed when the exporter is shutdown.
func WithExistingETWProvider(p *etw.Provider) Option {
	return func(h *handler) error {
		h.provider = p
		return nil
	}
}

// WithETWLevel specifies the [etw.Level] to use when writing errors as ETW events.
//
// The default is [etw.LevelError].
func WithETWLevel(l etw.Level) Option {
	return func(h *handler) error {
		h.level = l
		return nil
	}
}
