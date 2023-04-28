package jsonwriter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	tracesdk "go.opentelemetry.io/otel/sdk/trace"
)

var errNilWriter = errors.New("nil writer")

// not thread-safe.
// SpanProcessors (eg, BatchSpanProcessor) should handle synchronization
type exporter struct {
	w io.Writer
	j *json.Encoder
}

// New constructs a new Exporter and starts it.
func New(w io.Writer) (tracesdk.SpanExporter, error) {
	if w == nil {
		return nil, errNilWriter
	}
	j := json.NewEncoder(w)
	j.SetEscapeHTML(false)
	j.SetIndent("", "")

	return &exporter{
		w: w,
		j: j,
	}, nil
}

func (e *exporter) ExportSpans(ctx context.Context, spans []tracesdk.ReadOnlySpan) error {
	if e.w == nil || e.j == nil {
		// should not happen
		return fmt.Errorf("export error: %w", errNilWriter)
	}

	// todo (go1.20): switch to multierrors and handle individual errors
	// Errors will be sent to configured error handler
	var errs []error
	for _, ro := range spans {
		if err := ctx.Err(); err != nil {
			return err
		}
		span := FromReadOnly(ro)
		if err := e.j.Encode(span); err != nil {
			errs = append(errs, err)
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

func (e *exporter) Shutdown(ctx context.Context) error {
	e.w = nil
	e.j = nil

	return ctx.Err()
}
