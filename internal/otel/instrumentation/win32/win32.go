//go:build windows

package win32

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel/attribute"
	api "go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"golang.org/x/sys/windows"

	hcsmetric "github.com/Microsoft/hcsshim/internal/otel/metric"
)

// see:
// https://opentelemetry.io/docs/specs/otel/metrics/semantic_conventions/rpc-metrics/#rpc-client

// there isn't a general "unknown error" error code/ntstatus/hresult, and we aren't guaranteed the
// return value of a mkwinsyscall-generated function (from the go perspective) will be a [windows.Errno].
// so this key is used to distinguish between successful and failed win32 calls, absent a valid error code.
const successKey = attribute.Key("rpc.win32.success")

const errorCodeKey = attribute.Key("rpc.win32.error_code")

var rpcSystem = semconv.RPCSystemKey.String("win32")

// this will initially be created via the default (nop) MeterProvider, but so long as the
// global MeterProvider is initialized between this initialization and a [RecordDuration] call,
// then this will be recreated
var rpcDuration = hcsmetric.Int64Histogram("rpc.client.duration",
	api.WithDescription("Duration of outbound RPC requests"),
	api.WithUnit("ms"))

// RecordDuration records the duration of a Win32 API call to function in the specified library.
//
// At least one of function or library should be non-empty to identify the call.
func RecordDuration(ctx context.Context, library, function string, err error, duration time.Duration, options ...api.RecordOption) {
	attrs := make([]attribute.KeyValue, 0, 5)
	attrs = append(attrs, rpcSystem, successKey.Bool(err == nil))

	if library != "" {
		attrs = append(attrs, semconv.RPCService(library))
	}
	if function != "" {
		attrs = append(attrs, semconv.RPCMethod(function))
	}

	if e := windows.ERROR_SUCCESS; errors.As(err, &e) {
		attrs = append(attrs, errorCodeKey.Int64(int64(e)))
	}

	rpcDuration.Record(ctx, duration.Milliseconds(), append(options, api.WithAttributeSet(attribute.NewSet(attrs...)))...)
}
