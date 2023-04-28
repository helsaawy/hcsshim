//go:build windows

package etw

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Microsoft/go-winio/pkg/etw"
	"github.com/Microsoft/go-winio/pkg/guid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/Microsoft/hcsshim/internal/otel"
)

func init() {

}

// time.RFC3339Nano with nanoseconds padded using zeros to
// ensure the formatted time is always the same number of characters.
//
// Copied from [github.com/containerd/containerd/log.RFC3339NanoFixed]
//
// technically ISO 8601 and RFC 3339 are not equivalent, but ...
const iso8601 = "2006-01-02T15:04:05.000000000Z07:00"

// ErrLevelMismatch is returned when the configured ETW level for spans with an Error status is higher than
// the level for nominal spans.
var ErrLevelMismatch = errors.New("no ETW registered provider")

// not thread-safe
// SpanProcessors (eg, BatchSpanProcessor) should handle synchronization
type exporter struct {
	provider *etw.Provider

	closeProvider bool // if the provider is owned by the exporter
	setActivityID bool // set the (related) activity ID on the ETW event

	level      etw.Level
	errorLevel etw.Level

	// cache scopes and resources since they should not change
	scopes map[instrumentation.Scope]etw.FieldOpt
	rscs   map[attribute.Distinct]etw.FieldOpt // SDK should copy pointer to original resource struct
}

var _ tracesdk.SpanExporter = (*exporter)(nil)

// New returns an [tracesdk.SpanExporter] that exports to ETW.
//
// Span events and links are ignored.
func New(opts ...Option) (tracesdk.SpanExporter, error) {
	// C++ exporter writes as LevelAlways, .NET (Geneva) writes as LevelVerbose.
	// stick to prior (open census) behavior and use Info and Error
	e := &exporter{
		level:      etw.LevelInfo,
		errorLevel: etw.LevelError,
		scopes:     make(map[instrumentation.Scope]etw.FieldOpt),
		rscs:       make(map[attribute.Distinct]etw.FieldOpt),
	}

	for _, o := range opts {
		if err := o(e); err != nil {
			return nil, err
		}
	}

	if e.provider == nil {
		return nil, otel.ErrNoETWProvider
	}

	if uint8(e.level) < uint8(e.errorLevel) {
		return nil, fmt.Errorf("%w: error level (%d) is higher than normal level (%d)", ErrLevelMismatch, e.errorLevel, e.level)
	}

	return e, nil
}

// based on:
//https://github.com/open-telemetry/opentelemetry-cpp/blob/7cb7654552d68936d70986bc2ee67f3cc3e0b469/exporters/etw/include/opentelemetry/exporters/etw/etw_tracer.h#L235

func (e *exporter) ExportSpans(ctx context.Context, spans []tracesdk.ReadOnlySpan) error {
	if e.provider == nil {
		// should not happen
		return fmt.Errorf("export error: %w", otel.ErrNoETWProvider)
	}

	// todo (go1.20): switch to multierrors and handle individual errors
	// Errors will be sent to configured error handler
	var errs []error
	for _, span := range spans {
		if err := ctx.Err(); err != nil {
			return err
		}

		name := span.Name()
		sc := span.SpanContext()
		if !sc.IsValid() {
			errs = append(errs, fmt.Errorf("%w: %s", otel.ErrInvalidSpanContext, name))
			continue
		}

		spanID := sc.SpanID()
		traceID := sc.TraceID()
		pSpanID := span.Parent().SpanID()
		status := span.Status()
		isErrSpan := status.Code == codes.Error

		attributes := span.Attributes()

		opts := make([]etw.EventOpt, 0, 3) // level, activity ID, related activity ID

		lvl := e.level
		if isErrSpan {
			lvl = e.errorLevel
		}
		opts = append(opts, etw.WithLevel(lvl))

		if e.setActivityID {
			opts = append(opts, etw.WithActivityID(spanIDtoActivityID(spanID)))
			if pSpanID.IsValid() {
				opts = append(opts, etw.WithRelatedActivityID(spanIDtoActivityID(pSpanID)))
			}
		}

		// todo: events
		// todo: links

		// include several fields required by OTel spec
		// https://opentelemetry.io/docs/reference/specification/common/mapping-to-non-otlp/

		fields := make([]etw.FieldOpt, 0, len(attributes)+20) // rough pre-allocation guess, just to reserve room
		fields = append(fields,
			etw.StringField(fieldPayloadName, name),
			etw.StringField(fieldTraceID, traceID.String()),
			etw.StringField(fieldSpanID, spanID.String()),
			etw.StringField(fieldStartTime, span.StartTime().Format(iso8601)),
			etw.StringField(fieldEndTime, span.EndTime().Format(iso8601)),
			etw.Int64Field(fieldDuration, span.EndTime().Sub(span.StartTime()).Nanoseconds()),
			// coerce unspecified kinds to internal
			etw.StringField(fieldSpanKind, trace.ValidateSpanKind(span.SpanKind()).String()),
		)

		if pSpanID.IsValid() {
			fields = append(fields, etw.StringField(fieldSpanParentID, pSpanID.String()))
		}

		if n := span.DroppedAttributes(); n > 0 {
			fields = append(fields, etw.IntField(fieldDroppedAttributes, n))
		}
		if n := span.DroppedEvents(); n > 0 {
			fields = append(fields, etw.IntField(fieldDroppedEvents, n))
		}
		if n := span.DroppedLinks(); n > 0 {
			fields = append(fields, etw.IntField(fieldDroppedLinks, n))
		}

		// codes.Unset is the default and indicates that the operation was not validated
		if status.Code != codes.Unset {
			fields = append(fields,
				etw.StringField(fieldStatusCode, status.Code.String()),
			)
			if isErrSpan && status.Description != "" {
				// even if status.Description isn't empty, spec says its only valid for codes.Error
				fields = append(fields, etw.StringField(fieldStatusMessage, status.Description))
			}
		}

		for _, f := range []etw.FieldOpt{e.instrumentationScope(span), e.resource(span)} {
			if f != nil {
				fields = append(fields, f)
			}
		}

		// add attributes after span data, since ETW will prefer the first definition if there a conflict
		fields = append(fields, attributesToFields(attributes)...)

		if err := e.provider.WriteEvent(eventName, opts, fields); err != nil {
			// todo (go1.20): multierrors with both otel.ErrSpanExport and err
			errs = append(errs, fmt.Errorf("%v: span %s (%s): %w", otel.ErrSpanExport,
				name, spanContextString(traceID, spanID, pSpanID), err))
		}
	}

	switch n := len(errs); n {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
	}
	ss := make([]string, 0, len(errs))
	for _, e := range errs {
		ss = append(ss, e.Error())
	}
	return fmt.Errorf("multiple export errors: %s", strings.Join(ss, "; "))
}

func (e *exporter) Shutdown(ctx context.Context) (err error) {
	if e.provider == nil {
		return nil
	}

	if !e.closeProvider {
		err = e.provider.Close()
	}
	e.provider = nil

	if err != nil {
		return err
	}
	return ctx.Err()
}

func (e *exporter) instrumentationScope(s tracesdk.ReadOnlySpan) etw.FieldOpt {
	is := s.InstrumentationScope()
	if f, ok := e.scopes[is]; ok {
		return f
	}

	fields := make([]etw.FieldOpt, 0, 2)
	if is.Name != "" {
		fields = append(fields, etw.StringField("name", is.Name))
	}
	if is.Version != "" {
		fields = append(fields, etw.StringField("version", is.Version))
	}

	var f etw.FieldOpt
	if len(fields) > 0 {
		f = etw.Struct("otel.scope", fields...)
	}

	e.scopes[is] = f
	return f
}

func (e *exporter) resource(s tracesdk.ReadOnlySpan) etw.FieldOpt {
	rsc := s.Resource()
	k := rsc.Equivalent()
	if f, ok := e.rscs[k]; ok {
		return f
	}

	var f etw.FieldOpt
	if fs := attributesToFields(rsc.Attributes()); len(fs) > 0 {
		f = etw.Struct("otel.resource", fs...)
	}
	e.rscs[k] = f
	return f
}

func attributesToFields(attrs []attribute.KeyValue) []etw.FieldOpt {
	fields := make([]etw.FieldOpt, 0, len(attrs))

	for _, attr := range attrs {
		// AsInterface() will convert to the right field type based on OTel's supported field types,
		// and then etw.SmartField will do its own type-matching
		//
		// Should not receive an unknown value type.
		fields = append(fields, etw.SmartField(string(attr.Key), attr.Value.AsInterface()))
	}
	return fields
}

// simple string format for a traceID/spanID/parentSpanID triple for use in error strings
func spanContextString(tID trace.TraceID, sID, psID trace.SpanID) string {
	return tID.String() + "-" + sID.String() + "-" + psID.String()
}

// spanIDtoActivityID converts an 8 byte span ID to 16 byte Activity ID (GUID) by zero padding last 8 bytes.
// this is mostly cosmetic, and allows quickly searching for span IDs using activity id.
// This does not propagate activity ID to win32 calls, since that requires setting the thread activity ID
// using `EventActivityIdControl`.
//
// While the [W3C] recommends zero-padding on the left when creating trace IDs from smaller identifiers,
// it does not give recomendations for converting or padding span ID.
// Using trace ID as the GUID would not allow granularity for distinguishing which portions of a distributed
// trace spawned the API call.
// Therefore, the [C++ ETW OTel] convention of right-padding the span ID with zeros is used.
//
// [W3C]: https://www.w3.org/TR/trace-context/#interoperating-with-existing-systems-which-use-shorter-identifiers
// [C++ ETW OTel]: https://github.com/open-telemetry/opentelemetry-cpp/blob/7cb7654552d68936d70986bc2ee67f3cc3e0b469/exporters/etw/include/opentelemetry/exporters/etw/etw_config.h#L197
func spanIDtoActivityID(spanID trace.SpanID) guid.GUID {
	if !spanID.IsValid() {
		return guid.GUID{}
	}

	var x [16]byte
	copy(x[:], spanID[:])
	return guid.FromArray(x)
}
