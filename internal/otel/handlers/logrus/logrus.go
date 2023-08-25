// This package provides and [OTel error handler] that outputs via logrus.
//
// [OTel error handler]: https://pkg.go.dev/go.opentelemetry.io/otel#ErrorHandler
package logrus

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/Microsoft/hcsshim/internal/log"
)

// New creates a new [otel.ErrorHandler] to log errors raised durint OTel instrument creation/processing/export.
func New(opts ...Option) otel.ErrorHandler {
	c := newConfig()
	for _, o := range opts {
		o(&c)
	}

	return otel.ErrorHandlerFunc(func(err error) {
		// TODO: same enhancements proposed in etw handler (stacktrace and error type)

		// [WithFields] will create a copy of c.extra, so we don't need to worry about
		// copying it per call to prevent inadvertent modification
		logrus.WithFields(c.extra).WithError(err).Log(c.level, "OpenTelemetry error")
	})

}

type Option func(*config)

type config struct {
	level logrus.Level
	extra logrus.Fields
}

func newConfig() config {
	return config{
		level: logrus.ErrorLevel,
		extra: make(logrus.Fields),
	}
}

// WithResource specifies an OTel [resource.Resource] to append to the error message.
func WithResource(rsc *resource.Resource) Option {
	attr := rsc.Attributes()
	m := make(map[string]any, len(attr))
	for _, kv := range rsc.Attributes() {
		m[string(kv.Key)] = kv.Value.AsInterface()
	}
	return func(c *config) {
		if s := log.Format(context.Background(), m); s != "" {
			c.extra["otel.resource"] = s
		}
	}
}

// WithExtra specifies additional [logrus.Fields] to append to the error message.
func WithExtra(fields logrus.Fields) Option {
	return func(c *config) {
		for k, v := range fields {
			c.extra[k] = v
		}
	}
}

// WithLevel specifies the [logrus.Level] to use when writing errors.
//
// The default is [logrus.LevelError].
func WithLevel(l logrus.Level) Option {
	return func(c *config) {
		c.level = l
	}
}
