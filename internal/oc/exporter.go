package oc

import (
	"time"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

// LogrusExporter is an OpenCensus `trace.Exporter` that exports
// `trace.SpanData` to logrus output.
type LogrusExporter struct{}

var _ = (trace.Exporter)(&LogrusExporter{})

// ExportSpan exports `s` based on the the following rules:
//
// 1. All output will contain `s.Attributes`, `s.SpanKind`, `s.TraceID`,
// `s.SpanID`, and `s.ParentSpanID` for correlation
//
// 2. Any calls to .Annotate will not be supported.
//
// 3. The span itself will be written at `logrus.InfoLevel` unless
// `s.Status.Code != 0` in which case it will be written at `logrus.ErrorLevel`
// providing `s.Status.Message` as the error value.
func (le *LogrusExporter) ExportSpan(s *trace.SpanData) {
	if s.DroppedAnnotationCount > 0 {
		logrus.WithFields(logrus.Fields{
			"name":          s.Name,
			"dropped":       s.DroppedAttributeCount,
			"maxAttributes": len(s.Attributes),
		}).Warning("span had dropped attributes")
	}

	// Combine all span annotations with traceID, spanID, parentSpanID, and error
	entry := logrus.WithFields(logrus.Fields(s.Attributes))
	// Add fields in batch, rather than incrementally to avoid reallocating
	// entry.WithFields has overhead vs adding to entry.Data manually, but should be safer
	fs := logrus.Fields{
		"traceID":      s.TraceID.String(),
		"spanID":       s.SpanID.String(),
		"parentSpanID": s.ParentSpanID.String(),
		"startTime":    s.StartTime.Format(time.RFC3339Nano),
		"endTime":      s.EndTime.Format(time.RFC3339Nano),
		"duration":     s.EndTime.Sub(s.StartTime).String(),
		"name":         s.Name,
	}
	if s.Status.Code != 0 {
		fs[logrus.ErrorKey] = s.Status.Message
	}
	if sk := spanKindToString(s.SpanKind); sk != "" {
		fs["spanKind"] = sk
	}
	entry = entry.WithFields(fs)
	entry.Time = s.StartTime

	level := logrus.InfoLevel
	if s.Status.Code != 0 {
		level = logrus.ErrorLevel
	}

	entry.Log(level, "Span")
}
