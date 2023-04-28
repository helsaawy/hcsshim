package ttrpcinterceptor

import (
	"context"
	"net"
	"strconv"
	"strings"

	"github.com/containerd/ttrpc"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/Microsoft/hcsshim/internal/otel"
)

// based on otelgrpc interceptor
// https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc
// and this PR:
// https://github.com/containerd/ttrpc/pull/134

// todo: grpc interceptors add events with the request/response contents; switch to that

// ClientInterceptor returns a TTRPC unary client interceptor that automatically
// creates a new span for outgoing TTRPC calls, and passes the span context as
// metadata on the call.
func ClientInterceptor() ttrpc.UnaryClientInterceptor {
	return func(ctx context.Context, req *ttrpc.Request, resp *ttrpc.Response, info *ttrpc.UnaryClientInfo, inv ttrpc.Invoker) error {
		name, attrs := nameAndAttributes(ctx, info.FullMethod)
		ctx, span := otel.StartSpan(ctx, name, otel.WithClientSpanKind, trace.WithAttributes(attrs...))
		otel.InjectContext(ctx, requestCarrier{r: req})

		err := inv(ctx, req, resp)

		endSpan(span, err)

		return err
	}
}

// ServerInterceptor returns a TTRPC unary server interceptor that automatically
// creates a new span for incoming TTRPC calls, and parents the span to the
// span context received via metadata, if it exists.
func ServerInterceptor() ttrpc.UnaryServerInterceptor {
	return func(ctx context.Context, unmarshal ttrpc.Unmarshaler, info *ttrpc.UnaryServerInfo, method ttrpc.Method) (interface{}, error) {
		if md, ok := ttrpc.GetMetadata(ctx); ok {
			ctx = otel.ExtractContext(ctx, metadataCarrier{&md})
		}

		name, attrs := nameAndAttributes(ctx, info.FullMethod)
		ctx, span := otel.StartSpan(ctx, name, otel.WithServerSpanKind, trace.WithAttributes(attrs...))

		resp, err := method(ctx, func(req interface{}) error {
			err := unmarshal(req)
			if err == nil {
				otel.SetRPCRequestAttribute(ctx, req)
			}
			return err
		})

		otel.SetRPCResponseAttribute(ctx, resp)
		endSpan(span, err)

		return resp, err
	}
}

func nameAndAttributes(ctx context.Context, name string) (string, []attribute.KeyValue) {
	// method names should be of the form `/service.name/request`
	name = strings.TrimPrefix(name, "/")
	attrs := make([]attribute.KeyValue, 0, 5) // rpc system, service, & method; 2 peer info fields

	attrs = append(attrs, semconv.RPCSystemKey.String("ttrpc"))
	attrs = append(attrs, peerAttr(ctx)...)
	if svc, req, ok := strings.Cut(name, "/"); ok {
		attrs = append(attrs,
			semconv.RPCService(svc),
			semconv.RPCMethod(req),
		)
	}

	return strings.Join(strings.Split(name, "/"), "."), attrs
}

func endSpan(span trace.Span, err error) {
	// ttrpc uses grpc error/status codes
	c := status.Code(err)
	span.SetAttributes(attribute.Key("rpc.ttrpc.status_code").Int64(int64(c)))
	otel.SetSpanStatusAndEnd(span, err)
}

// peerAttr extrace peer info from the context and returns attributes about the address.
func peerAttr(ctx context.Context) []attribute.KeyValue {
	addr := ""
	if p, ok := peer.FromContext(ctx); ok {
		addr = p.Addr.String()
	}

	host, p, err := net.SplitHostPort(addr)
	if err != nil {
		return nil
	}

	if host == "" {
		host = "127.0.0.1"
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return nil
	}

	if ip := net.ParseIP(host); ip != nil {
		return []attribute.KeyValue{
			semconv.NetSockPeerAddr(host),
			semconv.NetSockPeerPort(port),
		}
	}
	return []attribute.KeyValue{
		semconv.NetPeerName(host),
		semconv.NetPeerPort(port),
	}
}
