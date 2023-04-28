package etw

// names based on OTel specification and OTLP convention
// https://opentelemetry.io/docs/reference/specification/common/mapping-to-non-otlp/
// https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/trace/v1/trace.proto#L80

const (
	fieldPayloadName  = "name"
	fieldTraceID      = "trace_id"
	fieldSpanID       = "span_id"
	fieldSpanParentID = "parent_span_id"
	fieldSpanKind     = "kind"

	fieldStartTime = "start_time"
	fieldEndTime   = "end_time"
	fieldDuration  = "duration"

	fieldStatusCode    = "otel.status_code"
	fieldStatusMessage = "otel.status_description"

	fieldDroppedAttributes = "otel.dropped_attributes_count"
	fieldDroppedEvents     = "otel.dropped_events_count"
	fieldDroppedLinks      = "otel.dropped_links_count"

	eventName = "Span" // ETW event name for Span
)
