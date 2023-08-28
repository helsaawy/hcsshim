//go:build windows

package etwmetric

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/Microsoft/go-winio/pkg/etw"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"

	hcsotel "github.com/Microsoft/hcsshim/internal/otel"
	oteletw "github.com/Microsoft/hcsshim/internal/otel/etw"
)

// TODO: export exemplars in when serializing datapoints (https://opentelemetry.io/docs/specs/otel/metrics/sdk/#exemplar)

// names (and inclusion of instrumentation scope and resource) based on OTel specification
// (and OTLP convention):
//
//  - https://opentelemetry.io/docs/reference/specification/common/mapping-to-non-otlp/
//  - https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/metrics/v1/metrics.proto

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

	fieldInstrumentationScope = "otel.scope"
	fieldSchemaURL            = "otel.schema_url"
)

// not thread-safe
// SpanProcessors (eg, BatchSpanProcessor) should handle synchronization
type exporter struct {
	provider *etw.Provider

	closeProvider bool // if the provider is owned by the exporter

	level etw.Level

	temporality metric.TemporalitySelector
	aggregation metric.AggregationSelector

	// cache scope and resources transformation since they should not change
	//
	// resources are serialized as a struct named [oteletw.FieldResource] to avoid conflicts (and clutter)
	// in the emitted  event, but instrumentation information is added to the event directly.
	// this is:
	//  1. to follow [OTel mapping recomendations]
	//  2. to streamline ETW processing based on instrumentation processing
	//
	// [OTel mapping recomendations]: https://opentelemetry.io/docs/specs/otel/common/mapping-to-non-otlp/#instrumentationscope

	scopes map[instrumentation.Scope][]etw.FieldOpt
	rscs   map[attribute.Distinct]etw.FieldOpt
}

var _ metric.Exporter = &exporter{}

func (e *exporter) Temporality(k metric.InstrumentKind) metricdata.Temporality {
	return e.temporality(k)
}

func (e *exporter) Aggregation(k metric.InstrumentKind) metric.Aggregation {
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
		scopes:      make(map[instrumentation.Scope][]etw.FieldOpt),
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

	if !e.provider.IsEnabledForLevel(e.level) {
		return nil
	}

	// todo (go1.20): switch to multierrors and handle individual errors
	// Errors will be sent to configured error handler
	var errs []error
	rsc := e.serializeResource(metrics.Resource)
	// OTLP specifies the schema url, and allows instrumentation scopes to override resource-level schema url.
	// follow that convention.
	// additionally, OTel convention for scope does not specify schema URL, so we add that as a dedicated field.
	rscURL := metrics.Resource.SchemaURL()

	for _, scope := range metrics.ScopeMetrics {
		is := e.serializeInstrumentationScope(scope.Scope)
		url := rscURL
		if scope.Scope.SchemaURL != "" {
			url = scope.Scope.SchemaURL
		}

		for _, m := range scope.Metrics {
			// short circut on context cancellation
			if err := ctx.Err(); err != nil {
				return err
			}

			opts := []etw.EventOpt{etw.WithLevel(e.level)}
			base := make([]etw.FieldOpt, 0, 5+len(is))
			base = append(base,
				etw.StringField(fieldName, m.Name),
				etw.StringField(fieldDescription, m.Description),
				etw.StringField(fieldUnit, m.Unit),
			)
			if url != "" {
				base = append(base, etw.StringField(fieldSchemaURL, url))
			}
			base = append(base, is...)
			if rsc != nil {
				base = append(base, rsc)
			}

			data, err := e.serializeData(m.Data)
			if err != nil {
				errs = append(errs, fmt.Errorf("metric data serialization for %q: %w", m.Name, err))
				continue
			}

			// each data point in the metric has unique attributes, so output each in a dedicated event
			for _, fs := range data {
				fields := make([]etw.FieldOpt, 0, len(base)+len(fs))
				fields = append(fields, base...)
				fields = append(fields, fs...)

				if err := e.provider.WriteEvent(eventName, opts, fields); err != nil {
					errs = append(errs, fmt.Errorf("metric export for %q: %w", m.Name, err))
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
func (e *exporter) serializeData(data metricdata.Aggregation) (fs [][]etw.FieldOpt, err error) {
	switch data := data.(type) {
	case metricdata.Sum[float64]:
		fs, err = serializeSum(data)
	case metricdata.Sum[int64]:
		fs, err = serializeSum(data)
	case metricdata.Gauge[float64]:
		fs, err = serializeGauge(data)
	case metricdata.Gauge[int64]:
		fs, err = serializeGauge(data)
	case metricdata.Histogram[int64]:
		fs, err = serializeHistogram(data)
	case metricdata.Histogram[float64]:
		fs, err = serializeHistogram(data)
	default:
		return nil, fmt.Errorf("unknown aggregation type: %T", data)
	}
	return fs, err
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

func serializeGauge[T int64 | float64](data metricdata.Gauge[T]) ([][]etw.FieldOpt, error) {
	return serializeDataPoints(data.DataPoints)
}

func serializeDataPoints[T int64 | float64](data []metricdata.DataPoint[T]) ([][]etw.FieldOpt, error) {
	fields := make([][]etw.FieldOpt, 0, len(data))
	for _, datum := range data {
		attrs := oteletw.SerializeAttributes(datum.Attributes.ToSlice())
		fs := make([]etw.FieldOpt, 0, 5+len(attrs)) // (start) time, value, temporality, monotonic
		fs = append(fs, serializeNum("value", datum.Value))

		if !datum.StartTime.IsZero() {
			fs = append(fs, etw.StringField("start_time", datum.StartTime.Format(iso8601)))
		}
		if !datum.Time.IsZero() {
			fs = append(fs, etw.StringField("time", datum.Time.Format(iso8601)))
		}

		fs = append(fs, attrs...)
		fields = append(fields, fs)
	}
	return fields, nil
}

func serializeHistogram[T int64 | float64](data metricdata.Histogram[T]) ([][]etw.FieldOpt, error) {
	fs, err := serializeHistogramDataPoints(data.DataPoints)
	if err != nil {
		return nil, err
	}
	for i := range fs {
		fs[i] = append(fs[i],
			etw.StringField("temporality", data.Temporality.String()),
		)
	}
	return fs, nil
}

func serializeHistogramDataPoints[T int64 | float64](data []metricdata.HistogramDataPoint[T]) ([][]etw.FieldOpt, error) {
	fields := make([][]etw.FieldOpt, 0, len(data))
	for _, datum := range data {
		attrs := oteletw.SerializeAttributes(datum.Attributes.ToSlice())
		fs := make([]etw.FieldOpt, 0, 10+len(attrs))

		fs = append(fs,
			etw.Float64Array("buckets", append(datum.Bounds, math.Inf(1))), // +inf bound is left un-added
			etw.Uint64Array("bucket_counts", datum.BucketCounts),
			etw.Uint64Field("count", datum.Count),
			serializeNum("sum", datum.Sum),
		)

		if v, ok := datum.Min.Value(); ok {
			fs = append(fs, serializeNum("min", v))
		}
		if v, ok := datum.Max.Value(); ok {
			fs = append(fs, serializeNum("max", v))
		}

		if !datum.StartTime.IsZero() {
			fs = append(fs, etw.StringField("start_time", datum.StartTime.Format(iso8601)))
		}
		if !datum.Time.IsZero() {
			fs = append(fs, etw.StringField("time", datum.Time.Format(iso8601)))
		}

		fs = append(fs, attrs...)
		fields = append(fields, fs)
	}
	return fields, nil
}

func serializeNum[T int64 | float64](n string, v T) etw.FieldOpt {
	// can't type switch on type parameter
	// https://github.com/golang/go/issues/45380

	switch v := any(v).(type) {
	case int64:
		return etw.Int64Field(n, v)
	case float64:
		return etw.Float64Field(n, v)
	}
	// shouldn't get here, but ...
	return etw.SmartField(n, v)
}

func (e *exporter) serializeResource(rsc *resource.Resource) etw.FieldOpt {
	k := rsc.Equivalent()
	if f, ok := e.rscs[k]; ok {
		return f
	}

	f := oteletw.SerializeResource(rsc)
	e.rscs[k] = f
	return f
}

// see: https://opentelemetry.io/docs/specs/otel/common/mapping-to-non-otlp/#instrumentationscope
func (e *exporter) serializeInstrumentationScope(is instrumentation.Scope) []etw.FieldOpt {
	if f, ok := e.scopes[is]; ok {
		return f
	}

	// don't need to report schema URL
	fields := make([]etw.FieldOpt, 0, 2)
	if is.Name != "" {
		fields = append(fields, etw.StringField(fieldInstrumentationScope+".name", is.Name))
	}
	if is.Version != "" {
		fields = append(fields, etw.StringField(fieldInstrumentationScope+".version", is.Version))
	}

	e.scopes[is] = fields
	return fields
}
