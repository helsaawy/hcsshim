// This package provides OpenTelemetry instrumentation for ttrpc servers, similar to the [otelgrpc package].
//
// [otelgrpc package]: https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc
package otelttrpc

import (
	"context"
	"strings"
	"time"

	"github.com/containerd/ttrpc"
	"go.opentelemetry.io/otel/attribute"
	api "go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc/status"

	hcsmetric "github.com/Microsoft/hcsshim/internal/otel/metric"
)

// based of otelgrpc (see doc comment) and this PR:
// https://github.com/containerd/ttrpc/pull/145

func UnaryServerInterceptor() ttrpc.UnaryServerInterceptor {
	// even though OTel recomendation is to use only base units (ie, seconds), their convention
	// for RPC servers is to use ms for request duration
	// (https://opentelemetry.io/docs/specs/otel/metrics/semantic_conventions/rpc-metrics/#rpc-server).
	//
	// The grpc interceptors follow this convention:
	// https://github.com/open-telemetry/opentelemetry-go-contrib/blob/8f53fc19a16c5bd0b1dc1254d5e41696a9ae262e/instrumentation/google.golang.org/grpc/otelgrpc/config.go#L75
	ttrpcDuration := hcsmetric.Int64Histogram("rpc.ttrpc.duration",
		api.WithDescription("duration of ttrpc requests"),
		api.WithUnit("ms"))

	return func(ctx context.Context, u ttrpc.Unmarshaler, usi *ttrpc.UnaryServerInfo, m ttrpc.Method) (r any, err error) {
		attrs := make([]attribute.KeyValue, 0, 4) // rpc system, service, method, & status
		attrs = append(attrs, semconv.RPCSystemKey.String("ttrpc"))
		// method names should be of the form `/service.name/request`
		if svc, req, ok := strings.Cut(strings.TrimPrefix(usi.FullMethod, "/"), "/"); ok {
			attrs = append(attrs,
				semconv.RPCService(svc),
				semconv.RPCMethod(req),
			)
		}

		defer func(t time.Time) {
			d := time.Since(t).Milliseconds()
			// ttrpc uses grpc error/status codes

			ttrpcDuration.Record(ctx, d, api.WithAttributeSet(attribute.NewSet(
				append(attrs, attribute.Key("rpc.ttrpc.status_code").Int64(int64(status.Code(err))))...,
			)))
		}(time.Now())

		return m(ctx, u)
	}
}

// ttrpc does not allow chaining interceptors
//
// based on:
// https://github.com/containerd/ttrpc/pull/152
// https://github.com/grpc/grpc-go/blob/23ac72b6454a2bcac32e19ccf501ca3a070f517c/server.go#L1184-L1197

// ChainUnaryServerInterceptors chains multiple [ttrpc.UnaryServerInterceptor] functions together
// by converting each to a [ttrpc.Method].
func ChainUnaryServerInterceptors(fs ...ttrpc.UnaryServerInterceptor) ttrpc.UnaryServerInterceptor {
	// we want to call the last interceptor first
	slices.Reverse(fs)

	return func(ctx context.Context, unmarshal ttrpc.Unmarshaler, info *ttrpc.UnaryServerInfo, method ttrpc.Method) (any, error) {
		for _, i := range fs {
			method = convertInterceptor(i, info, method)
		}
		return method(ctx, unmarshal)
	}
}

// convertInterceptor converts a ttrpc interceptor to a ttrpc method handler, so it can be chained with
// another interceptor.
func convertInterceptor(i ttrpc.UnaryServerInterceptor, info *ttrpc.UnaryServerInfo, method ttrpc.Method) ttrpc.Method {
	if i == nil {
		return method
	}
	return func(ctx context.Context, unmarshal func(any) error) (any, error) {
		return i(ctx, unmarshal, info, method)
	}
}
