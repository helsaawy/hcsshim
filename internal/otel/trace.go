package otel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Microsoft/hcsshim/internal/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Create an appropriate span name from the components.
//
// Component names are joined via "::".
func Name(names ...string) string {
	ns := make([]string, 0, len(names))
	// filter out empty strings
	for _, n := range names {
		if n != "" {
			ns = append(ns, n)
		}
	}
	return strings.Join(ns, "::")
}

func SetSpanStatusAndEnd(span trace.Span, err error, opts ...trace.SpanEndOption) {
	SetSpanStatus(span, err)
	span.End(opts...)
}

// SetSpanStatus sets the span status and records an error if err != nil.
// nop otherwise.
func SetSpanStatus(span trace.Span, err error) {
	if err == nil {
		span.SetStatus(codes.Ok, "")
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// StartSpanWithRemoteParent wraps "go.opentelemetry.io/otel/trace".Tracer.StartSpan, but sets
// the parent [trace.SpanContext] as the parent for the new span.
//
// See StartSpan for more information.
func StartSpanWithRemoteParent(ctx context.Context, name string, parent trace.SpanContext, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return StartSpan(trace.ContextWithSpanContext(ctx, parent), name, opts...)
}

// StartSpan wraps "go.opentelemetry.io/otel/trace".Tracer.StartSpan, but, if the span is sampling,
// adds a log entry to the context that points to the newly created span.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	ctx, s := tracer().Start(ctx, name, opts...)
	return ctx, s
}

//
// for RPC request and response attribute, the keys follow OTel semantic conventions,
// but there doesn't seems to be precident/recommendations for adding the request/response
// message as an attribute.
//

func SetRPCRequestAttribute(ctx context.Context, req any) {
	span := trace.SpanFromContext(ctx)
	if !(span.IsRecording() && span.SpanContext().IsValid()) {
		return
	}

	s := ""
	switch v := req.(type) {
	case proto.Message:
		b, err := protojson.Marshal(v)
		if err != nil {
			return
		}
		s = string(b)
	default:
		s = log.Format(ctx, v)
	}

	if s = strings.TrimSpace(s); s != "" {
		span.SetAttributes(attribute.Key("rpc.request").String(s))
	}
}

func SetRPCResponseAttribute(ctx context.Context, resp any) {
	span := trace.SpanFromContext(ctx)
	if !(span.IsRecording() && span.SpanContext().IsValid()) {
		return
	}

	s := ""
	switch v := resp.(type) {
	case *emptypb.Empty:
		return
	case proto.Message:
		b, err := protojson.Marshal(v)
		if err != nil {
			return
		}
		s = string(b)
	default:
		s = log.Format(ctx, v)
	}

	if s = strings.TrimSpace(s); s != "" {
		span.SetAttributes(attribute.Key("rpc.response").String(s))
	}
}

var WithServerSpanKind = trace.WithSpanKind(trace.SpanKindServer)
var WithClientSpanKind = trace.WithSpanKind(trace.SpanKindClient)

func Attribute(k string, v interface{}) attribute.KeyValue {
	// copied from github.com/containerd/containerd/tracing/helpers.go:any
	if v == nil {
		return attribute.String(k, "<nil>")
	}

	switch typed := v.(type) {
	case bool:
		return attribute.Bool(k, typed)
	case []bool:
		return attribute.BoolSlice(k, typed)
	case int:
		return attribute.Int(k, typed)
	case []int:
		return attribute.IntSlice(k, typed)
	case int8:
		return attribute.Int(k, int(typed))
	case []int8:
		ls := make([]int, 0, len(typed))
		for _, i := range typed {
			ls = append(ls, int(i))
		}
		return attribute.IntSlice(k, ls)
	case int16:
		return attribute.Int(k, int(typed))
	case []int16:
		ls := make([]int, 0, len(typed))
		for _, i := range typed {
			ls = append(ls, int(i))
		}
		return attribute.IntSlice(k, ls)
	case int32:
		return attribute.Int64(k, int64(typed))
	case []int32:
		ls := make([]int64, 0, len(typed))
		for _, i := range typed {
			ls = append(ls, int64(i))
		}
		return attribute.Int64Slice(k, ls)
	case int64:
		return attribute.Int64(k, typed)
	case []int64:
		return attribute.Int64Slice(k, typed)
	case float64:
		return attribute.Float64(k, typed)
	case []float64:
		return attribute.Float64Slice(k, typed)
	case string:
		return attribute.String(k, typed)
	case []string:
		return attribute.StringSlice(k, typed)
	}

	if stringer, ok := v.(fmt.Stringer); ok {
		return attribute.Stringer(k, stringer)
	}
	if b, err := json.Marshal(v); b != nil && err == nil {
		return attribute.String(k, string(b))
	}
	return attribute.String(k, fmt.Sprintf("%v", v))
}
