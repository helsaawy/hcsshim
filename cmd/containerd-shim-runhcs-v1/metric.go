//go:build windows

package main

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"time"

	"github.com/Microsoft/go-winio/pkg/etw"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"

	"github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/options"
	"github.com/Microsoft/hcsshim/internal/log"
	hcsotel "github.com/Microsoft/hcsshim/internal/otel"
	"github.com/Microsoft/hcsshim/internal/otel/exporters/etw/etwmetric"
	etwhandler "github.com/Microsoft/hcsshim/internal/otel/handlers/etw"
	logrushandler "github.com/Microsoft/hcsshim/internal/otel/handlers/logrus"
	hcsmetric "github.com/Microsoft/hcsshim/internal/otel/metric"
)

// TODO: instrument bridge and HCS RPC calls

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

	exp, err := metricExporter(ctx, conf)
	if err != nil {
		log.G(ctx).WithFields(logrus.Fields{
			logrus.ErrorKey: err,
		}).Warning("could not create OTel metrics exporter")
		return nil
	}

	interval := 5 * time.Minute
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

	// the read mem stats interval should be the max of the configured value and  reader interval,
	// but set it explicitly here just to be safe.
	if err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(interval)); err != nil {
		otel.Handle(fmt.Errorf("initialize runtime instrumentation: %w", err))
	}
	return cleanup
}

func metricExporter(ctx context.Context, conf *options.MetricsConfig) (exp metric.Exporter, err error) {
	et := conf.ExporterType
	switch et {
	case options.MetricsConfig_ETW:
		exp, err = etwmetric.New(etwmetric.WithExistingProvider(etwProvider))
	case options.MetricsConfig_OTLP:
		conf := conf.GetOtlpConfig()
		if conf == nil {
			err = fmt.Errorf("OTLP config cannot be nil")
			break
		}

		// based off of containerd tracing processor plugin
		// github.com/containerd/containerd/tracing/plugin/otlp.go
		switch p := conf.Protocol; p {
		case "", "http/protobuf":
			var u *url.URL
			u, err = url.Parse(conf.Endpoint)
			if err != nil {
				err = fmt.Errorf("invalid OTLP endpoint %q: %w", conf.Endpoint, err)
				break
			}
			opts := []otlpmetrichttp.Option{
				otlpmetrichttp.WithEndpoint(u.Host),
			}
			if u.Scheme == "http" {
				opts = append(opts, otlpmetrichttp.WithInsecure())
			}
			exp, err = otlpmetrichttp.New(ctx, opts...)
		case "grpc":
			opts := []otlpmetricgrpc.Option{
				otlpmetricgrpc.WithEndpoint(conf.Endpoint),
			}
			if conf.Insecure {
				opts = append(opts, otlpmetricgrpc.WithInsecure())
			}
			exp, err = otlpmetricgrpc.New(ctx, opts...)
		default:
			err = fmt.Errorf("unsupported OTLP protocol %s", p)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("OpenTelemetry %q metrics exporter: %w", et.String(), err)
	}

	log.G(ctx).WithField("config", log.Format(ctx, conf)).Debug("created OTel metrics exporter")
	return exp, nil
}
