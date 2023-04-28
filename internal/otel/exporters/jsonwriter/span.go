package jsonwriter

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Microsoft/hcsshim/internal/otel"
	"github.com/containerd/containerd/protobuf"
	typeurl "github.com/containerd/typeurl/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/anypb"
)

// Need a span struct to serialize and de-serialize across GCS to re-export via
// the SpanProcessors configured for the shim.
//
// [stdouttrace.Exporter] does serialize to JSON, but does not implement de-serialization,
// since it use [tracetest.SpanStub] internally (see below).
// stdouttrace: https://github.com/open-telemetry/opentelemetry-go/tree/main/exporters/stdout/stdouttrace
//
// OTLP ("go.opentelemetry.io/proto/otlp/trace/v1") is fairly difficult to transform back
// into a "go.opentelemetry.io/otel/sdk/trace".ReadOnlySpan.
// Would need to undo all the transformations done below:
// https://github.com/open-telemetry/opentelemetry-go/blob/main/exporters/otlp/otlptrace/internal/tracetransform/span.go
// https://github.com/open-telemetry/opentelemetry-proto-go/blob/main/otlp/trace/v1/trace.pb.go
//
// Implement our own span, based on [tracetest.SpanStub].
// [tracetest.SpanStub] does not serialize well either, since [*resource.Resource],
// [attribute.KeyValue] and [trace.SpanContext] either have private fields or do not implement
// [json.Unmarshall].
//
// tracetest.SpanStub: https://github.com/open-telemetry/opentelemetry-go/blob/main/sdk/trace/tracetest/span.go

// todo: Events, Links

type Span struct {
	Name                 string                `json:",omitempty"`
	TraceID              TraceID               `json:",omitempty"`
	SpanID               SpanID                `json:",omitempty"`
	ParentSpanID         SpanID                `json:",omitempty"`
	TraceState           string                `json:",omitempty"`
	Kind                 trace.SpanKind        `json:",omitempty"`
	StartTimeUnixNano    int64                 `json:",omitempty"`
	EndTimeUnixNano      int64                 `json:",omitempty"`
	Attributes           []KeyValue            `json:",omitempty"`
	Status               tracesdk.Status       `json:",omitempty"`
	DroppedAttributes    int64                 `json:",omitempty"`
	Resource             Resource              `json:",omitempty"`
	InstrumentationScope instrumentation.Scope `json:",omitempty"`
}

func FromReadOnly(ro tracesdk.ReadOnlySpan) Span {
	if ro == nil {
		return Span{}
	}

	sc := ro.SpanContext()
	return Span{
		Name:                 ro.Name(),
		TraceID:              TraceID(sc.TraceID()),
		SpanID:               SpanID(sc.SpanID()),
		ParentSpanID:         SpanID(ro.Parent().SpanID()),
		TraceState:           sc.TraceState().String(),
		Kind:                 trace.ValidateSpanKind(ro.SpanKind()),
		StartTimeUnixNano:    ro.StartTime().UnixNano(),
		EndTimeUnixNano:      ro.EndTime().UnixNano(),
		Attributes:           keyValueList(ro.Attributes()),
		Status:               ro.Status(),
		DroppedAttributes:    int64(ro.DroppedAttributes()),
		Resource:             newResource(ro.Resource()),
		InstrumentationScope: ro.InstrumentationScope(),
	}
}

func (s *Span) Snapshot() tracesdk.ReadOnlySpan {
	// use tracetest to create a ReadOnlySpan, rather than implementing our own

	// ignore tracestate parse errors, and leave the SpanContext's trace state as blank
	ts, _ := trace.ParseTraceState(s.TraceState)
	ro := &tracetest.SpanStub{
		Name: s.Name,
		SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    trace.TraceID(s.TraceID),
			SpanID:     trace.SpanID(s.SpanID),
			TraceState: ts,
			TraceFlags: trace.FlagsSampled,
		}),
		Parent: trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    trace.TraceID(s.TraceID),
			SpanID:     trace.SpanID(s.ParentSpanID),
			TraceFlags: trace.FlagsSampled,
		}),
		SpanKind:               trace.ValidateSpanKind(s.Kind),
		Attributes:             toAttributes(s.Attributes),
		DroppedAttributes:      int(s.DroppedAttributes),
		Status:                 s.Status,
		StartTime:              time.Unix(0, s.StartTimeUnixNano),
		EndTime:                time.Unix(0, s.EndTimeUnixNano),
		Resource:               s.Resource.toResource(),
		InstrumentationLibrary: s.InstrumentationScope,
	}

	return ro.Snapshot()
}

func (s *Span) Valid() bool {
	return !(s.Name == "" || s.TraceID == TraceID([16]byte{}) || s.SpanID == SpanID([8]byte{}) || s.Kind == trace.SpanKindUnspecified)
}

// Need custom types that can be (de)serialized across the GCS

type TraceID [16]byte

func (x TraceID) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(x[:]))
}

func (x *TraceID) UnmarshalJSON(b []byte) error {
	s := ""
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	if hex.DecodedLen(len(s)) != len(x) {
		return fmt.Errorf("invalid trace ID string: %s", s)
	}

	bb, err := hex.DecodeString(s)
	if err != nil {
		return err
	}

	copy(x[:], bb)
	return nil
}

type SpanID [8]byte

func (x SpanID) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(x[:]))
}

func (x *SpanID) UnmarshalJSON(b []byte) error {
	s := ""
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	if hex.DecodedLen(len(s)) != len(x) {
		return fmt.Errorf("invalid trace ID string: %s", s)
	}

	bb, err := hex.DecodeString(s)
	if err != nil {
		return err
	}

	copy(x[:], bb)
	return nil
}

type Resource struct {
	Attributes []KeyValue
	SchemaURL  string
}

// map[attribute.Distinct][]KeyValue
// cache [resource.Resource] attributes
var rscs sync.Map

func newResource(rsc *resource.Resource) Resource {
	d := rsc.Equivalent()
	if r, ok := rscs.Load(d); ok {
		return r.(Resource)
	}

	r := Resource{
		Attributes: keyValueList(rsc.Attributes()),
		SchemaURL:  rsc.SchemaURL(),
	}
	rscs.Store(d, r)
	return r
}

func (r *Resource) toResource() *resource.Resource {
	// todo: add cache, like in [newResource]?
	return resource.NewWithAttributes(r.SchemaURL,
		toAttributes(r.Attributes)...,
	)
}

func keyValueList(attrs []attribute.KeyValue) []KeyValue {
	kvs := make([]KeyValue, 0, len(attrs))
	for _, a := range attrs {
		if kv := toKeyValue(a); kv.valid() {
			kvs = append(kvs, kv)
		}
	}
	return kvs
}

func toAttributes(kvs []KeyValue) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(kvs))
	for _, kv := range kvs {
		a := kv.attribute()
		if a.Valid() {
			attrs = append(attrs, a)
		}
	}
	return attrs
}

// Can only be string, bool, float64, int64, or an array of those.
// https://opentelemetry.io/docs/reference/specification/common/#attribute

type KeyValue struct {
	Key   string
	Value Value
}

var (
	_ fmt.Stringer     = (*KeyValue)(nil)
	_ json.Marshaler   = (*KeyValue)(nil)
	_ json.Unmarshaler = (*KeyValue)(nil)
)

func (kv *KeyValue) String() string {
	return fmt.Sprintf(`KeyValue{Key: %s, Value: %v}`, kv.Key, kv.Value.any())
}

// for marshalling and unmarshalling via typeurl
type kvInternal struct {
	Key   string
	Value *anypb.Any
}

func (kv *KeyValue) MarshalJSON() (_ []byte, err error) {
	x := &kvInternal{
		Key: kv.Key,
	}

	x.Value, err = protobuf.MarshalAnyToProto(kv.Value)
	if err != nil {
		return nil, fmt.Errorf("marshal Value to typeurl: %w", err)
	}
	return json.Marshal(x)
}

func (kv *KeyValue) UnmarshalJSON(b []byte) error {
	x := &kvInternal{
		Value: &anypb.Any{},
	}

	if err := json.Unmarshal(b, x); err != nil {
		return err
	}

	kv.Key = x.Key

	v, err := typeurl.UnmarshalAny(x.Value)
	if err != nil {
		return fmt.Errorf("unmarshal Value from typeurl: %w", err)
	}

	var ok bool
	kv.Value, ok = v.(Value)
	if !ok {
		return fmt.Errorf("invalid Value type: %T", v)
	}

	return nil
}

func toKeyValue(a attribute.KeyValue) KeyValue {
	if !a.Valid() {
		return KeyValue{}
	}

	return KeyValue{
		Key:   string(a.Key),
		Value: newValue(a.Value),
	}
}

func (kv *KeyValue) attribute() attribute.KeyValue {
	return otel.Attribute(kv.Key, kv.Value.any())
}

func (kv *KeyValue) valid() bool {
	return kv.Key != "" && kv.Value != nil
}

func init() {
	typeurl.Register(&StringValue{},
		"github.com/Microsoft/hcsshim/internal/otel/exporters/jsonwriter", "StringValue")
	typeurl.Register(&BoolValue{},
		"github.com/Microsoft/hcsshim/internal/otel/exporters/jsonwriter", "BoolValue")
	typeurl.Register(&Float64Value{},
		"github.com/Microsoft/hcsshim/internal/otel/exporters/jsonwriter", "Float64Value")
	typeurl.Register(&Int64Value{},
		"github.com/Microsoft/hcsshim/internal/otel/exporters/jsonwriter", "Int64Value")

	typeurl.Register(&StringSliceValue{},
		"github.com/Microsoft/hcsshim/internal/otel/exporters/jsonwriter", "StringSliceValue")
	typeurl.Register(&BoolSliceValue{},
		"github.com/Microsoft/hcsshim/internal/otel/exporters/jsonwriter", "BoolSliceValue")
	typeurl.Register(&Float64SliceValue{},
		"github.com/Microsoft/hcsshim/internal/otel/exporters/jsonwriter", "Float64SliceValue")
	typeurl.Register(&Int64SliceValue{},
		"github.com/Microsoft/hcsshim/internal/otel/exporters/jsonwriter", "Int64SliceValue")
}

type Value interface{ any() any }

type StringValue struct {
	Value string
}

func (v *StringValue) any() any { return v.Value }

type BoolValue struct {
	Value bool
}

func (v *BoolValue) any() any { return v.Value }

type Float64Value struct {
	Value float64
}

func (v *Float64Value) any() any { return v.Value }

type Int64Value struct {
	Value int64
}

func (v *Int64Value) any() any { return v.Value }

type StringSliceValue struct {
	Value []string
}

func (v *StringSliceValue) any() any { return v.Value }

type BoolSliceValue struct {
	Value []bool
}

func (v *BoolSliceValue) any() any { return v.Value }

type Float64SliceValue struct {
	Value []float64
}

func (v *Float64SliceValue) any() any { return v.Value }

type Int64SliceValue struct {
	Value []int64
}

func (v *Int64SliceValue) any() any { return v.Value }

func newValue(v attribute.Value) Value {
	switch v.Type() {
	case attribute.BOOL:
		return &BoolValue{Value: v.AsBool()}
	case attribute.BOOLSLICE:
		return &BoolSliceValue{Value: v.AsBoolSlice()}
	case attribute.INT64:
		return &Int64Value{Value: v.AsInt64()}
	case attribute.INT64SLICE:
		return &Int64SliceValue{Value: v.AsInt64Slice()}
	case attribute.FLOAT64:
		return &Float64Value{Value: v.AsFloat64()}
	case attribute.FLOAT64SLICE:
		return &Float64SliceValue{Value: v.AsFloat64Slice()}
	case attribute.STRING:
		return &StringValue{Value: v.AsString()}
	case attribute.STRINGSLICE:
		return &StringSliceValue{Value: v.AsStringSlice()}
	default:
	}
	// should only happen when value is [attribute.INVALID]
	return nil
}
