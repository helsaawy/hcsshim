//go:build windows

package etw

import (
	"github.com/Microsoft/go-winio/pkg/etw"
	"go.opentelemetry.io/otel"

	hcsotel "github.com/Microsoft/hcsshim/internal/otel"
)

// .NET OTel SDK sets the event name to "OpenTelemetry-Sdk".
// We do something similar here.
//
// https://github.com/open-telemetry/opentelemetry-dotnet/blob/main/src/OpenTelemetry/Internal/OpenTelemetrySdkEventSource.cs
const name = "OpenTelemetry.Error"

type handler struct {
	provider *etw.Provider
	level    etw.Level
}

var _ otel.ErrorHandler = (*handler)(nil)

// New creates a new [ote.ErrorHandler] to log errors raised span processing/export.
//
// Since [otel.ErrorHandler] does not expose a Close/Shutdown function, this error handler
// expects the ETW provider to exist until the global [otel.TraceProvider] is shutdown.
func New(opts ...Option) (otel.ErrorHandler, error) {
	h := &handler{
		level: etw.LevelError,
	}
	for _, o := range opts {
		if err := o(h); err != nil {
			return nil, err
		}
	}

	if h.provider == nil {
		return nil, hcsotel.ErrNoETWProvider
	}
	return h, nil
}

func (h *handler) Handle(e error) {
	// TODO: switch based on error type and write out more information/stack-trace (if available)
	// ignore errors; theres no one to report too
	_ = h.provider.WriteEvent(name,
		[]etw.EventOpt{etw.WithLevel(h.level)},
		[]etw.FieldOpt{etw.StringField("Error", e.Error())},
	)
}
