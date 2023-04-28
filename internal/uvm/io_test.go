//go:build windows

package uvm

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/Microsoft/hcsshim/internal/otel/exporters/jsonwriter"
	"github.com/sirupsen/logrus"
)

func TestParseGCSLogEntry(t *testing.T) {
	// test log and span parsing with the same `x`
	x := gcsLogOrSpan{
		vmid: t.Name(),
	}

	//
	// test span entry
	//

	tID := trace.TraceID([16]byte{0x20, 0xaf, 0xc2, 0x2d, 0x82, 0x90, 0xac, 0x7f, 0x8b, 0x35, 0xf7, 0x9f, 0x7b, 0xa9, 0x1a, 0x9b})
	sID := trace.SpanID([8]byte{0x19, 0x6d, 0x90, 0xc1, 0xc0, 0x22, 0xc0, 0xd8})
	psID := trace.SpanID([8]byte{0x9c, 0xb1, 0x02, 0x78, 0x7a, 0x77, 0x62, 0x7c})
	s := jsonwriter.FromReadOnly((&tracetest.SpanStub{
		Name: "span.name",
		SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    tID,
			SpanID:     sID,
			TraceFlags: trace.FlagsSampled,
		}),
		Parent: trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    tID,
			SpanID:     psID,
			TraceFlags: trace.FlagsSampled,
		}),
		SpanKind: trace.SpanKindConsumer,
		Attributes: []attribute.KeyValue{
			attribute.Int64("count", 16),
			attribute.Float64Slice("some numbers", []float64{3, 2.4, 19, 58.33}),
			attribute.String("stringThing", "this is a string"),
		},
		DroppedAttributes: 4,
		Status: tracesdk.Status{
			Code:        codes.Error,
			Description: t.Name() + " failed somehow",
		},
		StartTime: time.Now().Add(-2 * time.Second),
		EndTime:   time.Now(),
		Resource: resource.NewWithAttributes(semconv.SchemaURL,
			semconv.TelemetrySDKLanguageGo,
			semconv.ServiceName("testing"),
		),
		InstrumentationLibrary: instrumentation.Scope{
			Name:      "github.com/Microsoft/hcsshim/internal/uvm",
			SchemaURL: semconv.SchemaURL,
		},
	}).Snapshot())

	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatalf("marshal span: %v", err)
	}

	if err := json.Unmarshal(b, &x); err != nil {
		t.Fatalf("unmarshall span to gcsLogOrSpan: %v", err)
	}

	if !x.s.Valid() {
		t.Fatalf("invalid span: %#+v", x.s)
	}
	if !reflect.DeepEqual(s, x.s) {
		t.Fatalf("%v != %v", s, x.s)
	}

	if len(x.e.Fields) != 0 {
		t.Fatalf("log entry fields should be empty; is %v", x.e.Fields)
	}
	if x.e.Message != "" {
		t.Fatalf("log message should be empty; is %s", x.e.Message)
	}

	//
	// test reset
	//

	x.reset()
	if len(x.s.Attributes) != 0 {
		t.Fatalf("log attributes should be empty; is %v", x.s.Attributes)
	}
	if x.s.Name != "" {
		t.Fatalf("span name should be empty; is %s", x.s.Name)
	}
	if len(x.e.Fields) != 0 {
		t.Fatalf("log entry fields should be empty; is %v", x.e.Fields)
	}
	if x.e.Message != "" {
		t.Fatalf("log message should be empty; is %s", x.e.Message)
	}

	//
	// test log entry
	//

	e := gcsLogEntry{
		gcsLogEntryStandard: gcsLogEntryStandard{
			Time:    time.Now(),
			Message: "this is a log",
			Level:   logrus.InfoLevel,
		},
		Fields: map[string]interface{}{
			"field1": "is a field",
			"number": int64(3),
		},
	}
	m := map[string]any{
		"msg":   e.Message,
		"time":  e.Time,
		"level": e.Level,
	}
	for k, v := range e.Fields {
		m[k] = v
	}

	b, err = json.Marshal(&m)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}

	if err := json.Unmarshal(b, &x); err != nil {
		t.Fatalf("unmarshall entry to gcsLogOrSpan: %v", err)
	}

	// time equality has to be via [time.Equal]
	if !e.Time.Equal(x.e.Time) {
		t.Fatalf("time %v != %v", e.Time, x.e.Time)
	}
	e.Time = time.Time{}
	x.e.Time = time.Time{}
	if !reflect.DeepEqual(e, x.e) {
		t.Fatalf("%v != %v", e.gcsLogEntryStandard, x.e.gcsLogEntryStandard)
	}

	if len(x.s.Attributes) != 0 {
		t.Fatalf("log attributes should be empty; is %v", x.s.Attributes)
	}
	if x.s.Name != "" {
		t.Fatalf("span name should be empty; is %s", x.s.Name)
	}
}
