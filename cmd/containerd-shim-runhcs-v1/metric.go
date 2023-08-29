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
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

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
	conf := opts.GetOtelConfig().GetMetrics()
	if conf == nil || !conf.Enabled {
		log.G(ctx).Debug("OTel metrics are disabled")
		return nil
	}

	// we currently only need the resource and error handler for metrics
	// if OTel traces/logs are added, move the init to a dedicated sync.Once
	rsc := initOTel(ctx, name, version)

	exp, err := metricExporter(ctx, conf)
	if err != nil {
		log.G(ctx).WithFields(logrus.Fields{
			logrus.ErrorKey: err,
		}).Warning("could not create OTel metric exporter")
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
		"exporter":        conf.ExporterType.String(),
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
	et := conf.GetExporterType()
	switch et {
	case options.OTelExporterType_ETW:
		exp, err = etwmetric.New(etwmetric.WithExistingProvider(etwProvider))
	case options.OTelExporterType_OTLP:
		conf := conf.GetOtlpConfig()
		if conf == nil {
			err = fmt.Errorf("OTLP config cannot be nil")
			break
		}

		// based off of containerd tracing processor plugin
		// github.com/containerd/containerd/tracing/plugin/otlp.go
		switch p := conf.Protocol; p {
		case "", "http/protobuf":
			var opts []otlpmetrichttp.Option
			if conf.Endpoint != "" {
				var u *url.URL
				u, err = url.Parse(conf.Endpoint)
				if err != nil {
					err = fmt.Errorf("invalid OTLP endpoint %q: %w", conf.Endpoint, err)
					break
				}
				opts = append(opts, otlpmetrichttp.WithEndpoint(u.Host))
				if u.Scheme == "http" {
					opts = append(opts, otlpmetrichttp.WithInsecure())
				}
			}
			exp, err = otlpmetrichttp.New(ctx, opts...)
		case "grpc":
			var opts []otlpmetricgrpc.Option
			if conf.Endpoint != "" {
				opts = append(opts, otlpmetricgrpc.WithEndpoint(conf.Endpoint))
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
		return nil, fmt.Errorf("OpenTelemetry %q metric exporter: %w", et.String(), err)
	}

	log.G(ctx).WithField("type", et.String()).Debug("created OTel metric exporter")
	return exp, nil
}

func initOTel(ctx context.Context, name, version string) *resource.Resource {
	// first create the resource to initialize the error handler with, so we can identify
	rsc, rErr := hcsotel.DefaultResource(ctx, name, version,
		semconv.ServiceInstanceID(idFlag),
		semconv.ServiceNamespace(namespaceFlag),
		semconv.NetSockHostAddr(addressFlag),
	)
	if rErr != nil {
		log.G(ctx).WithError(rErr).Warn("OTel resource creation errors")
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		log.G(ctx).WithField("resource", log.Format(ctx, rsc)).Debug("created OTel resource")
	}

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

	if logrus.IsLevelEnabled(logrus.DebugLevel) {

		log.G(ctx).WithField("type", reflect.Indirect(reflect.ValueOf(h)).Type().String()).Debug("created OTel error handler")
	}

	if rErr != nil {
		otel.Handle(err)
	}

	return rsc
}
