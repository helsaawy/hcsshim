package otel

import (
	"sync"

	tracesdk "go.opentelemetry.io/otel/sdk/trace"
)

// register SpanProcessors so Spans received out-of-band (eg, those serialized from the GCS)
// can be re-exported

var spanProcessors = struct {
	m   sync.Mutex
	sps []tracesdk.SpanProcessor
}{
	sps: make([]tracesdk.SpanProcessor, 0),
}

func RegisterSpanProcessor(sp tracesdk.SpanProcessor) {
	if sp == nil {
		return
	}

	spanProcessors.m.Lock()
	defer spanProcessors.m.Unlock()

	spanProcessors.sps = append(spanProcessors.sps, sp)
}

func ExportSpan(s tracesdk.ReadOnlySpan) {
	spanProcessors.m.Lock()
	defer spanProcessors.m.Unlock()

	for _, sp := range spanProcessors.sps {
		sp.OnEnd(s)
	}
}
