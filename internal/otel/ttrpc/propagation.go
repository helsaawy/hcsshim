package ttrpcinterceptor

import (
	"strings"

	"github.com/containerd/ttrpc"
	"go.opentelemetry.io/otel/propagation"
)

// duplicate metadata keys are allowed, so similar to how the grpc carriers work, get returns the first
// and set appends.
//
// https://github.com/open-telemetry/opentelemetry-go-contrib/blob/instrumentation/google.golang.org/grpc/otelgrpc/v0.40.0/instrumentation/google.golang.org/grpc/otelgrpc/metadata_supplier.go

type requestCarrier struct {
	r *ttrpc.Request
}

var _ propagation.TextMapCarrier = requestCarrier{}

func (c requestCarrier) Get(key string) string {
	key = strings.ToLower(key)
	for _, kv := range c.r.GetMetadata() {
		if kv.Key == key {
			return kv.Key
		}
	}

	return ""
}

func (c requestCarrier) Set(key, value string) {
	key = strings.ToLower(key)
	c.r.Metadata = append(c.r.Metadata, &ttrpc.KeyValue{Key: key, Value: value})
}

func (c requestCarrier) Keys() []string {
	md := c.r.GetMetadata()
	keys := make([]string, 0, len(md)) // duplicates are allowed
	for _, kv := range md {
		keys = append(keys, kv.Key)
	}
	return keys
}

// this is for server requests, settings has no effect
type metadataCarrier struct {
	md *ttrpc.MD
}

var _ propagation.TextMapCarrier = metadataCarrier{}

func (c metadataCarrier) Get(key string) string {
	if vs, ok := c.md.Get(key); ok {
		return vs[0]
	}
	return ""
}

func (c metadataCarrier) Set(key, value string) {
	c.md.Set(key, value)
}

func (c metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(*c.md))
	for key := range *c.md {
		keys = append(keys, key)
	}
	return keys
}
