//go:build windows

package etwmetric

import (
	"context"
	"fmt"
	"strings"

	"github.com/Microsoft/go-winio/pkg/etw"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"

	hcsotel "github.com/Microsoft/hcsshim/internal/otel"
)

// time.RFC3339Nano with nanoseconds padded using zeros to
// ensure the formatted time is always the same number of characters.
//
// Copied from [github.com/containerd/containerd/log.RFC3339NanoFixed]
//
// technically ISO 8601 and RFC 3339 are not equivalent, but ...
const iso8601 = "2006-01-02T15:04:05.000000000Z07:00"

// ETW constants
const (
	eventName = "Metric"

	fieldName        = "name"
	fieldStartTime   = "start_time"
	fieldTime        = "time"
	fieldDescription = "description"
	fieldUnit        = "unit"
	fieldValue       = "value"
)

// not thread-safe
// SpanProcessors (eg, BatchSpanProcessor) should handle synchronization
type exporter struct {
	provider *etw.Provider

	closeProvider bool // if the provider is owned by the exporter

	level etw.Level

	temporality metric.TemporalitySelector
	aggregation metric.AggregationSelector

	// cache scopes and resources since they should not change
	scopes map[instrumentation.Scope]etw.FieldOpt
	rscs   map[attribute.Distinct]etw.FieldOpt // SDK should copy pointer to original resource struct
}

var _ metric.Exporter = &exporter{}

func (e *exporter) Temporality(k metric.InstrumentKind) metricdata.Temporality {
	return e.temporality(k)
}

func (e *exporter) Aggregation(k metric.InstrumentKind) aggregation.Aggregation {
	return e.aggregation(k)
}

// New returns a [metric.Exporter] that exports metrics to ETW.
func New(opts ...Option) (metric.Exporter, error) {
	// C++ exporter writes as LevelAlways, .NET (Geneva) writes as LevelVerbose.
	// stick to prior (open census) behavior and use Info and Error
	e := &exporter{
		level:       etw.LevelInfo,
		temporality: metric.DefaultTemporalitySelector,
		aggregation: metric.DefaultAggregationSelector,
		scopes:      make(map[instrumentation.Scope]etw.FieldOpt),
		rscs:        make(map[attribute.Distinct]etw.FieldOpt),
	}

	for _, o := range opts {
		if err := o(e); err != nil {
			return nil, err
		}
	}

	if e.provider == nil {
		return nil, hcsotel.ErrNoETWProvider
	}

	return e, nil
}

func (e *exporter) Export(ctx context.Context, metrics *metricdata.ResourceMetrics) error {
	if e.provider == nil {
		// should not happen
		return hcsotel.ErrNoETWProvider
	}

	// todo (go1.20): switch to multierrors and handle individual errors
	// Errors will be sent to configured error handler
	var errs []error
	rsc := e.resource(metrics.Resource)

	for _, scope := range metrics.ScopeMetrics {
		is := e.instrumentationScope(scope.Scope)
		for _, m := range scope.Metrics {
			// short circut on context cancellation
			if err := ctx.Err(); err != nil {
				return err
			}

			opts := []etw.EventOpt{etw.WithLevel(e.level)}
			// rough pre-allocation guess
			base := []etw.FieldOpt{
				etw.StringField(fieldName, m.Name),
				etw.StringField(fieldDescription, m.Description),
				etw.StringField(fieldUnit, m.Unit),
			}

			data, err := e.serializeData(m.Data)
			if err != nil {
				errs = append(errs, fmt.Errorf("metric data serialization: %s: %w", m.Name, err))
				continue
			}

			// each data point in the metric gets its own event, with unique attributes
			for _, fs := range data {
				fields := make([]etw.FieldOpt, 0, len(base)+len(fs)+2)
				fields = append(fields, base...)
				fields = append(fields, fs...)

				// add instrumantation scope and resource metrics data
				for _, f := range []etw.FieldOpt{is, rsc} {
					if f != nil {
						fields = append(fields, f)
					}
				}

				if err := e.provider.WriteEvent(eventName, opts, fields); err != nil {
					errs = append(errs, fmt.Errorf("metric export: %s: %w", m.Name, err))
				}
			}
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

func (e *exporter) ForceFlush(context.Context) error {
	// nop; we don't hold any data to flush
	return nil
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

// return an array of attributes for the different data points
func (*exporter) serializeData(data metricdata.Aggregation) (fs [][]etw.FieldOpt, err error) {
	switch a := data.(type) {
	case metricdata.Sum[float64]:
		fs, err = serializeSum(a)
	case metricdata.Sum[int64]:
		fs, err = serializeSum(a)
	case metricdata.Gauge[float64]:
		fs, err = serializeGauge(a)
	case metricdata.Gauge[int64]:
		fs, err = serializeGauge(a)
	case metricdata.Histogram[int64], metricdata.Histogram[float64]:
		// TODO
		return nil, fmt.Errorf("histograms are currently unsupported")
	default:
		return nil, fmt.Errorf("unknown aggregation type: %T", a)
	}
	return fs, err
}

func serializeGauge[T int64 | float64](data metricdata.Gauge[T]) ([][]etw.FieldOpt, error) {
	return serializeDataPoints(data.DataPoints)
}

func serializeSum[T int64 | float64](data metricdata.Sum[T]) ([][]etw.FieldOpt, error) {
	fs, err := serializeDataPoints(data.DataPoints)
	if err != nil {
		return nil, err
	}
	for i := range fs {
		fs[i] = append(fs[i],
			etw.StringField("temporality", data.Temporality.String()),
			etw.BoolField("monotonic", data.IsMonotonic),
		)
	}
	return fs, nil
}

func serializeDataPoints[T int64 | float64](data []metricdata.DataPoint[T]) ([][]etw.FieldOpt, error) {
	// todo: exemplars (as array?)

	fields := make([][]etw.FieldOpt, 0, len(data))
	for _, datum := range data {
		fs := make([]etw.FieldOpt, 0, 10)
		switch v := any(datum.Value).(type) {
		case int64:
			fs = append(fs, etw.Int64Field("value", v))
		case float64:
			fs = append(fs, etw.Float64Field("value", v))
		}
		if !datum.StartTime.IsZero() {
			fs = append(fs, etw.StringField("start_time", datum.StartTime.Format(iso8601)))
		}
		if !datum.Time.IsZero() {
			fs = append(fs, etw.StringField("time", datum.Time.Format(iso8601)))
		}
		fs = append(fs, attributesToFields(datum.Attributes.ToSlice())...)
		fields = append(fields, fs)
	}
	return fields, nil
}

func (e *exporter) resource(rsc *resource.Resource) etw.FieldOpt {
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

func (e *exporter) instrumentationScope(is instrumentation.Scope) etw.FieldOpt {
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
