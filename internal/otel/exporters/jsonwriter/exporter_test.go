package jsonwriter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"go.opentelemetry.io/otel/trace"

	hcsotel "github.com/Microsoft/hcsshim/internal/otel"
)

func TestKeyValueJSON(t *testing.T) {
	tests := []KeyValue{
		{
			Key:   "astring",
			Value: &StringValue{Value: "string"},
		},

		{
			Key:   "some strings",
			Value: &StringSliceValue{Value: []string{"testing", "string"}},
		},
		{
			Key:   "mask",
			Value: &BoolSliceValue{Value: []bool{}},
		},
		{
			Key:   "a better mask",
			Value: &BoolSliceValue{Value: []bool{true, false, true, true, false, false}},
		},
		{
			Key:   "count",
			Value: &Int64Value{Value: 16},
		},
		{
			Key:   "numbers",
			Value: &Float64SliceValue{Value: []float64{3, 2.4, 19, 58.33}},
		},
	}
	for _, kv := range tests {
		kv := kv
		t.Run(fmt.Sprint(kv.Key), func(t *testing.T) {
			b, err := json.Marshal(&kv)
			if err != nil {
				t.Fatalf("json marshal: %v", err)
			}

			kv2 := KeyValue{}
			if err := json.Unmarshal(b, &kv2); err != nil {
				t.Fatalf("json unmarshal: %v", err)
			}

			if !reflect.DeepEqual(kv, kv2) {
				t.Fatalf("%s != %s", kv.String(), kv2.String())
			}
		})
	}
}

func TestJSONMarshal(t *testing.T) {
	tID := trace.TraceID([16]byte{0x20, 0xaf, 0xc2, 0x2d, 0x82, 0x90, 0xac, 0x7f, 0x8b, 0x35, 0xf7, 0x9f, 0x7b, 0xa9, 0x1a, 0x9b})
	sID := trace.SpanID([8]byte{0x19, 0x6d, 0x90, 0xc1, 0xc0, 0x22, 0xc0, 0xd8})
	psID := trace.SpanID([8]byte{0x9c, 0xb1, 0x02, 0x78, 0x7a, 0x77, 0x62, 0x7c})
	ts, err := trace.ParseTraceState("testing=bob:hi;otherstate:slightlylongervalue")
	if err != nil {
		t.Fatalf("trace state: %v", err)
	}

	ro := (&tracetest.SpanStub{
		Name: "span.name",
		SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    tID,
			SpanID:     sID,
			TraceState: ts,
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
			Name:      "github.com/Microsoft/hcsshim/internal/otel/exporters/jsonwriter",
			SchemaURL: semconv.SchemaURL,
		},
	}).Snapshot()

	span := FromReadOnly(ro)
	b, err := json.Marshal(span)
	if err != nil {
		t.Fatalf("marshal span: %v", err)
	}

	var span2 Span
	if err := json.Unmarshal(b, &span2); err != nil {
		t.Fatalf("unmarshal span: %v", err)
	}

	if !span2.Valid() {
		t.Fatalf("invalid span: %#+v", span2)
	}

	if !reflect.DeepEqual(span, span2) {
		t.Fatalf("%v != %v", span, span2)
	}
}

func TestExporter(t *testing.T) {
	ctx := context.Background()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe failed: %v", err)
	}

	exporter, err := New(w)
	if err != nil {
		t.Fatalf("create exporter: %v", err)
	}

	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		t.Errorf("otel error: %v", err)
	}))

	cleanup, err := hcsotel.InitializeProvider(
		tracesdk.WithBatcher(exporter),
		tracesdk.WithSampler(tracesdk.AlwaysSample()),
		tracesdk.WithResource(resource.NewWithAttributes(semconv.SchemaURL,
			semconv.TelemetrySDKLanguageGo,
			semconv.ServiceName(t.Name()),
			semconv.ServiceVersion("2.3"),
		)),
	)
	if err != nil {
		t.Fatalf("initialize provider: %v", err)
	}

	type record struct {
		attrs []attribute.KeyValue
		err   string
	}
	records := make(map[string]record, 0)

	f := func(ctx context.Context, i int) (err error) {
		r := rand.Intn(1000)
		n := fmt.Sprintf("%s.f.%d", t.Name(), i)
		attrs := []attribute.KeyValue{
			hcsotel.Attribute("i", i),
			hcsotel.Attribute("r", r),
		}

		_, span := hcsotel.StartSpan(ctx, n,
			trace.WithAttributes(attrs...),
		)
		defer func() { hcsotel.SetSpanStatusAndEnd(span, err) }()
		defer func() {
			r := record{
				attrs: attrs,
			}
			if err != nil {
				r.err = err.Error()
			}
			records[n] = r
		}()
		time.Sleep(time.Duration(r) * time.Millisecond)

		if i%2 != 0 {
			return fmt.Errorf("uneven number %d", i)
		}
		return nil
	}

	func() {
		n := t.Name()
		attrs := []attribute.KeyValue{
			hcsotel.Attribute("number", 12.23),
			hcsotel.Attribute("name", "a name"),
			hcsotel.Attribute("Another name", "bob"),
			hcsotel.Attribute("ints", []int64{2, 3, 16, 58}),
		}
		ctx, span := hcsotel.StartSpan(ctx, n, trace.WithAttributes(attrs...))
		defer func() { hcsotel.SetSpanStatusAndEnd(span, nil) }()

		records[n] = record{attrs: attrs}

		t.Log("running")
		time.Sleep(time.Millisecond * 50)

		for i := 0; i <= 3; i++ {
			i := i
			t.Logf("calling %d...", i)
			_ = f(ctx, i)
		}
	}()

	done := make(chan struct{})
	go func() {
		_ = cleanup(ctx)
		_ = w.Close()
		close(done)
	}()

	// drain the io pipe
	spans := make([]Span, 0)
	j := json.NewDecoder(r)
	for {
		s := Span{}
		if err := j.Decode(&s); err != nil {
			if !errors.Is(err, io.EOF) {
				t.Fatalf("json decoder: %v", err)
			}
			break
		}
		t.Logf("decoded span: %s", s.Name)
		spans = append(spans, s)
	}

	<-done

	if len(spans) != len(records) {
		t.Errorf("got %d spans, wanted %d", len(spans), len(records))
	}

	for _, s := range spans {
		r, ok := records[s.Name]
		if !ok {
			t.Errorf("missing span %s", s.Name)
		}

		if !s.Valid() {
			t.Fatalf("invalid span: %#+v", s)
		}
		if attrs := toAttributes(s.Attributes); !reflect.DeepEqual(r.attrs, attrs) {
			t.Errorf("got attributes %#+v; wanted %#+v", attrs, r.attrs)
		}
		if d := s.Status.Description; d != r.err {
			t.Errorf("got description %#+v; wanted %#+v", d, r.err)
		}
	}
}
