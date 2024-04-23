//go:build windows

package computecore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.opencensus.io/trace"
	"golang.org/x/sys/windows"

	hcsschema "github.com/Microsoft/hcsshim/internal/hcs/schema2"
	"github.com/Microsoft/hcsshim/internal/interop"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/oc"
	"github.com/Microsoft/hcsshim/internal/timeout"
)

// TODO:  HcsSetComputeSystemCallback & HcsSetProcessCallback

type (
	// Handle to a compute system.
	HCSSystem windows.Handle

	// Handle to a process running in a compute system.
	HCSProcess windows.Handle
)

// HCSContext corresponds to a `void* context` parameter that allows for an arbitrary payload
// containing compute system-, process-, or operation-specific data to be passed to callbacks.
//
// It is not compatible with [context.Context].
type HCSContext uintptr

//go:generate go run github.com/Microsoft/go-winio/tools/mkwinsyscall -output zsyscall_windows.go ./*.go

// 	HRESULT WINAPI
// 	HcsEnumerateComputeSystems(
// 	    _In_opt_ PCWSTR        query,
// 	    _In_     HCS_OPERATION operation
// 	    );
//
//sys hcsEnumerateComputeSystems(query string, operation HCSOperation) (hr error) = computecore.HcsEnumerateComputeSystems?

func EnumerateComputeSystems(ctx context.Context, op hcsOperation, query *hcsschema.SystemQuery) (properties []hcsschema.Properties, err error) {
	ctx, cancel := context.WithTimeout(ctx, timeout.SyscallWatcher)
	defer cancel()

	ctx, span := oc.StartSpan(ctx, computecoreSpanName("HcsEnumerateComputeSystems"), oc.WithClientSpanKind)
	defer func() {
		if len(properties) != 0 {
			span.AddAttributes(trace.StringAttribute("properties", log.Format(ctx, properties)))
		}
		oc.SetSpanStatus(span, err)
		span.End()
	}()

	q := ""
	if query != nil {
		q, err = encode(query)
		if err != nil {
			return nil, err
		}
		span.AddAttributes(trace.StringAttribute("query", q))
	}

	return runOperation[[]hcsschema.Properties](
		ctx,
		op,
		func(_ context.Context, op hcsOperation) (err error) {
			return hcsEnumerateComputeSystems(q, op)
		},
	)
}

//  HRESULT WINAPI
//  HcsSetComputeSystemCallback(
//      _In_ HCS_SYSTEM computeSystem,
//      _In_ HCS_EVENT_OPTIONS callbackOptions,
//      _In_opt_ const void* context,
//      _In_ HCS_EVENT_CALLBACK callback
//      );
//
//sys hcsSetComputeSystemCallback(computeSystem HCSSystem, callbackOptions HCSEventOptions, context HCSContext, callback hcsEventCallbackUintptr) (hr error) = computecore.HcsSetComputeSystemCallback?

// SetCallback assigns a callback to handle events for the compute system.
func (s HCSSystem) SetCallback(ctx context.Context, options HCSEventOptions, hcsCtx HCSContext, callback HCSEventCallback) (err error) {
	_, span := oc.StartSpan(ctx, computecoreSpanName("HcsSetComputeSystemCallback"), oc.WithClientSpanKind)
	defer func() {
		oc.SetSpanStatus(span, err)
		span.End()
	}()
	ptr := callback.asCallback()
	span.AddAttributes(
		trace.StringAttribute("options", options.String()),
		trace.Int64Attribute("handle", int64(s)),
		trace.Int64Attribute("context", int64(hcsCtx)),
		trace.Int64Attribute("callback", int64(ptr)),
	)

	return hcsSetComputeSystemCallback(s, options, hcsCtx, ptr)
}

//  HRESULT WINAPI
//  HcsSetProcessCallback(
//      _In_ HCS_PROCESS process,
//      _In_ HCS_EVENT_OPTIONS callbackOptions,
//      _In_ void* context,
//      _In_ HCS_EVENT_CALLBACK callback
//      );
//
//sys hcsSetProcessCallback(process HCSProcess, callbackOptions HCSEventOptions, context HCSContext, callback hcsEventCallbackUintptr) (hr error) = computecore.HcsSetProcessCallback?

// SetCallback assigns a callback to handle events for the compute system.
func (p HCSProcess) SetCallback(ctx context.Context, options HCSEventOptions, hcsCtx HCSContext, callback HCSEventCallback) (err error) {
	_, span := oc.StartSpan(ctx, computecoreSpanName("HcsSetProcessCallback"), oc.WithClientSpanKind)
	defer func() {
		oc.SetSpanStatus(span, err)
		span.End()
	}()
	ptr := callback.asCallback()
	span.AddAttributes(
		trace.StringAttribute("options", options.String()),
		trace.Int64Attribute("handle", int64(p)),
		trace.Int64Attribute("context", int64(hcsCtx)),
		trace.Int64Attribute("callback", int64(ptr)),
	)

	return hcsSetProcessCallback(p, options, hcsCtx, ptr)
}

func runOperation[T any](
	ctx context.Context,
	op hcsOperation,
	f func(context.Context, hcsOperation) error,
) (v T, err error) {
	if err := f(ctx, op); err != nil {
		return v, err
	}

	result, err := op.WaitForOperationResult(ctx)
	if err != nil {
		//  WaitForOperationResult should (attempt to) parse result as [hcsschema.ResultError]
		// so just return results
		return v, err
	}
	err = json.Unmarshal([]byte(result), &v)
	return v, err
}

// bufferToString converts the `PWSTR` buffer to a string, and then frees the original buffer.
func bufferToString(buffer *uint16) string {
	if buffer == nil {
		return ""
	}
	return interop.ConvertAndFreeCoTaskMemString(buffer)
}

func encode(v any) (string, error) {
	// TODO: pool of encoders/buffers
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "")

	if err := enc.Encode(v); err != nil {
		return "", fmt.Errorf("json encoding: %w", err)
	}

	// encoder.Encode appends a newline to the end
	return strings.TrimSpace(buf.String()), nil
}

func validHandle(h windows.Handle) bool {
	return h != 0 && h != windows.InvalidHandle
}

func computecoreSpanName(names ...string) string {
	s := make([]string, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n != "" {
			s = append(s, n)
		}
	}
	return strings.Join(append([]string{"computecore"}, s...), "::")
}
