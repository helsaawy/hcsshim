package oc

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

var (
	DefaultSampler = trace.AlwaysSample()

	// TraceLevelSampler samples if the logging level is "Trace" or higher
	TraceLevelSampler = LoggingLevelSampler(logrus.TraceLevel)
)

func WithTraceLevelSampler(o *trace.StartOptions) { // trace.StartOption
	o.Sampler = TraceLevelSampler
}

func LoggingLevelSampler(lvl logrus.Level) trace.Sampler {
	return func(_ trace.SamplingParameters) trace.SamplingDecision {
		b := logrus.GetLevel() >= lvl
		return trace.SamplingDecision{Sample: b}
	}
}

// SetSpanStatus sets `span.SetStatus` to the proper status depending on `err`. If
// `err` is `nil` assumes `trace.StatusCodeOk`.
func SetSpanStatus(span *trace.Span, err error) {
	status := trace.Status{}
	if err != nil {
		// TODO: JTERRY75 - Handle errors in a non-generic way
		status.Code = trace.StatusCodeUnknown
		status.Message = err.Error()
	}
	span.SetStatus(status)
}

/*
 * convenience wrappers
 */

// StartSpan wraps go.opencensus.io/trace.StartSpan, but explicitly sets the
// sampler to DefaultSampler. This will override other trace.StartOptions
// and the sampling of the parent span.
//
// If the parent span may have been started with StartTraceSpan, use this to
// force sampling.
func StartSpan(ctx context.Context, name string, o ...trace.StartOption) (context.Context, *trace.Span) {
	o = append(o, trace.WithSampler(DefaultSampler))
	ctx, s := trace.StartSpan(ctx, name, o...)

	return ctx, s
}

// StartTraceSpan is similar to StartSpan, but uses TraceLevelSampler, which
// disables sampling on this span and its children if the logging
// level is less than logrus.Trace.
//
// The returned span will still propagate a SpanContext, and its children can be
// sampled and exporeted if started by StartSpan
func StartTraceSpan(ctx context.Context, name string, o ...trace.StartOption) (context.Context, *trace.Span) {
	o = append(o, WithTraceLevelSampler)
	ctx, s := trace.StartSpan(ctx, name, o...)

	return ctx, s
}

var WithServerSpanKind = trace.WithSpanKind(trace.SpanKindServer)
var WithClientSpanKind = trace.WithSpanKind(trace.SpanKindServer)

func spanKindToString(sk int) string {
	switch sk {
	case trace.SpanKindClient:
		return "client"
	case trace.SpanKindServer:
		return "server"
	default:
		return ""
	}
}
