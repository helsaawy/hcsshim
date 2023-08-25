//go:build windows

package main

import (
	"context"
	"reflect"
	"strings"
	"time"

	"github.com/Microsoft/go-winio/pkg/etw"
	"github.com/containerd/ttrpc"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	api "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"google.golang.org/grpc/status"

	"github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/options"
	"github.com/Microsoft/hcsshim/internal/log"
	hcsotel "github.com/Microsoft/hcsshim/internal/otel"
	"github.com/Microsoft/hcsshim/internal/otel/exporters/etw/etwmetric"
	etwhandler "github.com/Microsoft/hcsshim/internal/otel/handlers/etw"
	logrushandler "github.com/Microsoft/hcsshim/internal/otel/handlers/logrus"
	hcsmetric "github.com/Microsoft/hcsshim/internal/otel/metric"
)

// TODO:
// - add auto-counter for HCS code

func startMetrics(ctx context.Context, opts *options.Options, name, version string) func(context.Context) error {
	conf := opts.GetMetricsConfig()
	if conf == nil || !conf.Enabled {
		log.G(ctx).Info("skipping OTel metrics")
		return nil
	}

	// we currently only need the error handler for metrics
	// if OTel traces/logs are added, move the handler init to a dedicated sync.Once
	rsc := hcsotel.DefaultResource(name, version,
		semconv.ServiceInstanceID(idFlag),
		semconv.ServiceNamespace(namespaceFlag),
		semconv.NetSockHostAddr(addressFlag),
	)

	// set the error handler, this will write any internal OTel errors to ETW
	var h otel.ErrorHandler
	var err error
	if h, err = etwhandler.New(
		etwhandler.WithProvider(etwProvider),
		etwhandler.WithLevel(etw.LevelWarning),
		etwhandler.WithResource(rsc),
	); err != nil {
		log.G(ctx).WithFields(logrus.Fields{
			logrus.ErrorKey: err,
			"provider":      etwProvider.String(),
		}).Warning("could not create OTel ETW error handler")
		// fall back on logrus handler
		h = logrushandler.New(
			logrushandler.WithLevel(logrus.WarnLevel),
			logrushandler.WithResource(rsc),
		)
	}
	otel.SetErrorHandler(h)

	// TODO: conf.ExporterType and OTLP config
	exp, err := etwmetric.New(
		etwmetric.WithExistingProvider(etwProvider),
		// histograms are for durations, in seconds; set more appropriate boundaries boundaries
		etwmetric.WithAggregationSelector(
			func(ik metric.InstrumentKind) aggregation.Aggregation {
				switch ik {
				case metric.InstrumentKindHistogram:
					return aggregation.ExplicitBucketHistogram{
						Boundaries: []float64{0, 0.005, 0.01, 0.25, 0.50, 0.75, 1, 5, 10, 30, 60},
					}
				}
				return metric.DefaultAggregationSelector(ik)
			},
		),
	)
	if err != nil {
		log.G(ctx).WithFields(logrus.Fields{
			logrus.ErrorKey: err,
			"provider":      etwProvider.String(),
		}).Warning("could not create OTel metric ETW exporter")
		return nil
	}

	interval := 1 * time.Minute
	// interval := 5 * time.Minute
	if d := time.Duration(conf.ExportIntervalSecs) * time.Second; d > 0 {
		interval = d
		log.G(ctx).WithField("interval", interval.String()).Warning("overriding exporter read interval")
	}

	cleanup, err := hcsmetric.InitializeProvider(
		metric.WithReader(metric.NewPeriodicReader(exp, metric.WithInterval(interval))),
		metric.WithResource(rsc),
	)
	entry := log.G(ctx).WithFields(logrus.Fields{
		"reader-interval": interval.Seconds(),
		"exporter":        reflect.TypeOf(exp).Name(),
	})
	if err != nil {
		entry.WithError(err).Warning("could not initialize OTel meter provider")
		return nil
	}
	entry.Info("started OTel meter provider")

	hcsmetric.InitializeRuntimeInstruments()
	return cleanup
}

func ttrpcMetricInterceptor() ttrpc.UnaryServerInterceptor {
	// ttrpc counters and histograms
	// if this is called before the MeterProvider is is initialized,
	// they will first be created as nop instruments, and then recreated with valid implementations.
	//
	// see: https://pkg.go.dev/go.opentelemetry.io/otel#GetMeterProvider
	ttrpcCount := hcsmetric.Int64Counter("rpc.ttrpc.count",
		api.WithDescription("number of ttrpc requests received"),
		api.WithUnit("{count}"))
	ttrpcDuration := hcsmetric.Float64Histogram("rpc.ttrpc.duration",
		api.WithDescription("duration of ttrpc requests"),
		api.WithUnit("s"))

	return func(ctx context.Context, u ttrpc.Unmarshaler, usi *ttrpc.UnaryServerInfo, m ttrpc.Method) (any, error) {
		start := time.Now()
		r, err := m(ctx, u)
		d := time.Since(start)

		attrs := make([]attribute.KeyValue, 0, 4) // rpc system, service, method, & status
		attrs = append(attrs, semconv.RPCSystemKey.String("ttrpc"))

		// method names should be of the form `/service.name/request`
		if svc, req, ok := strings.Cut(strings.TrimPrefix(usi.FullMethod, "/"), "/"); ok {
			attrs = append(attrs,
				semconv.RPCService(svc),
				semconv.RPCMethod(req),
			)
		}

		// ttrpc uses grpc error/status codes
		attrs = append(attrs, attribute.Key("rpc.ttrpc.status_code").Int64(int64(status.Code(err))))

		set := attribute.NewSet(attrs...)
		ttrpcCount.Add(ctx, 1, api.WithAttributeSet(set))
		ttrpcDuration.Record(ctx, d.Seconds(), api.WithAttributeSet(set))

		return r, err
	}
}
