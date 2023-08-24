package metric

import (
	"context"
	"fmt"
	"runtime"

	"go.opentelemetry.io/otel"
	api "go.opentelemetry.io/otel/metric"
)

// TODO (helsaawy): https://pkg.go.dev/runtime/metrics

// we want to leak this/keep it global to reuse across metric callbacks (they are guaranteed to
// be called synchronously)
var _memStats = runtime.MemStats{}

func InitializeRuntimeInstruments() {
	// these are safe to define globally and be run during the `start` and `close` commands, since they
	// will nop if no global [metric.MeterProvider] is configured ([otel.SetMeterProvider]) or if no readers are
	// added to the provider.
	//
	// https://pkg.go.dev/go.opentelemetry.io/otel#GetMeterProvider

	// naming/format based on examples/guidance here:
	// https://opentelemetry.io/docs/specs/otel/metrics/semantic_conventions/runtime-environment-metrics
	Int64ObservableGauge("process.runtime.go.goroutine.count",
		api.WithDescription("number of goroutines"),
		api.WithUnit("{count}"),
		api.WithInt64Callback(func(_ context.Context, o api.Int64Observer) error {
			o.Observe(int64(runtime.NumGoroutine()))
			return nil
		}),
	)

	Int64ObservableGauge("process.runtime.go.cgo_calls",
		api.WithDescription("number of cgo calls made"),
		api.WithUnit("{count}"),
		api.WithInt64Callback(func(_ context.Context, o api.Int64Observer) error {
			o.Observe(int64(runtime.NumCgoCall()))
			return nil
		}),
	)

	memSys := Int64ObservableGauge("process.runtime.go.memory.sys",
		api.WithDescription("The total bytes of memory obtained from the OS"),
		api.WithUnit("By"))
	memTotal := Int64ObservableGauge("process.runtime.go.memory.total_alloc",
		api.WithDescription("The cumulative bytes allocated for heap objects"),
		api.WithUnit("By"))
	memHeap := Int64ObservableGauge("process.runtime.go.memory.heap_alloc",
		api.WithDescription("The bytes of allocated heap objects"),
		api.WithUnit("By"))
	memMalloc := Int64ObservableGauge("process.runtime.go.memory.mallocs",
		api.WithDescription("The cumulative count of heap objects allocated"),
		api.WithUnit("{count}"))
	memFree := Int64ObservableGauge("process.runtime.go.memory.frees",
		api.WithDescription("The cumulative count of heap objects freed"),
		api.WithUnit("{count}"))

	gcFrac := Float64ObservableGauge("process.runtime.go.gc.cpu_fraction",
		api.WithDescription("The fraction of this program's available CPU time used by the GC."),
		api.WithUnit("{fraction}"))
	gcCount := Int64ObservableGauge("process.runtime.go.gc.cycles",
		api.WithDescription("The number of completed GC cycles"),
		api.WithUnit("{count}"))

	if _, err := Meter().RegisterCallback(
		func(_ context.Context, o api.Observer) error {
			// This call does work
			runtime.ReadMemStats(&_memStats)

			o.ObserveInt64(memSys, int64(_memStats.Sys))
			o.ObserveInt64(memTotal, int64(_memStats.TotalAlloc))
			o.ObserveInt64(memHeap, int64(_memStats.HeapAlloc))
			o.ObserveInt64(memMalloc, int64(_memStats.Mallocs))
			o.ObserveInt64(memFree, int64(_memStats.Frees))
			o.ObserveFloat64(gcFrac, _memStats.GCCPUFraction)
			o.ObserveInt64(gcCount, int64(_memStats.NumGC))

			return nil
		},
		memSys,
		memTotal,
		memHeap,
		memMalloc,
		memFree,
		gcFrac,
		gcCount,
	); err != nil {
		otel.Handle(fmt.Errorf("register memory statistics callback: %w", err))
	}
}
