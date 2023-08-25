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
	extra    []etw.FieldOpt
}

var _ otel.ErrorHandler = (*handler)(nil)

// New creates a new [otel.ErrorHandler].
//
// Since [otel.ErrorHandler] does not expose a Close/Shutdown function, this error handler
// expects the ETW provider's lifetime to contain the global OTel providers' lifespans
// (ie, ["go.opentelemetry.io/otel/trace".TracerProvider] and ["go.opentelemetry.io/otel/metric".MeterProvider]),
// and persist after all OTel operations.
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
	// TODO: check if error has more information/stack-trace and write that out
	//  https://opentelemetry.io/docs/specs/semconv/exceptions/exceptions-spans/#stacktrace-representation
	// TODO: if error type is meaningfull (ie, not [fmt.wrapError] or [errors.errorString]), provide "exception.type"

	// ignore [WriteEvent] errors; theres no one to report them to
	_ = h.provider.WriteEvent(name,
		[]etw.EventOpt{etw.WithLevel(h.level)},
		append(
			[]etw.FieldOpt{etw.StringField("exception.message", e.Error())},
			h.extra...,
		),
	)
}
