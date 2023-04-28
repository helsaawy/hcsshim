// This package provides an OpenTelemetry exporter for ETW.
//
// Based on:
//   - [C++ OTel] ETW Exporter
//   - [.NET OTel Geneva] Exporter
//
// NOTE:
// While based on the C++ implementation, that library implements a dedicated trace provider
// which allows firing off dedicated ETW TraceLogging [Activity/Start] and [Activity/Stop] events
// (with the appropriate ETW opcodes) separate from the ETW event created with the full span properties.
//
// [C++ OTel]: https://github.com/open-telemetry/opentelemetry-cpp/tree/main/exporters/etw
// [.NET OTel Geneva]: https://github.com/open-telemetry/opentelemetry-dotnet-contrib/tree/main/src/OpenTelemetry.Exporter.Geneva
// [Activity/Start]: https://github.com/open-telemetry/opentelemetry-cpp/blob/7cb7654552d68936d70986bc2ee67f3cc3e0b469/exporters/etw/include/opentelemetry/exporters/etw/etw_tracer.h#L557-L564
// [Activity/Stop]: https://github.com/open-telemetry/opentelemetry-cpp/blob/7cb7654552d68936d70986bc2ee67f3cc3e0b469/exporters/etw/include/opentelemetry/exporters/etw/etw_tracer.h#L325-L334
package etw

// todo: create dedicated "go.opentelemetry.io/otel/sdk/trace".SpanProcessor (instead of BatchProcessor)
// so that we can override OnStart & OnEnd to output activity start and stop ETW events
