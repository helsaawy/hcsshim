package log

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

type entryContextKeyType int

const _entryContextKey entryContextKeyType = iota

var (
	// L is the default, blank logging entry. WithField and co. all return a copy
	// of the original entry, so this will not leak fields between calls.
	//
	// Do NOT modify fields directly, as that will corrupt state for all users and
	// is not thread safe.
	L = logrus.NewEntry(logrus.StandardLogger())

	// G is an alias for GetEntry
	G = GetEntry

	// S is an alias for SetEntry
	S = SetEntry

	// U is an alias for UpdateContext
	U = UpdateContext
)

// GetContext returns a `logrus.Entry` stored in context (if any), appended with
// the `TraceID, SpanID` from `ctx` if `ctx` contains an OpenCensus `trace.Span`.
//
// Unlike FromContext, this will overwrite any tracing information already
// present in the entry.
func GetEntry(ctx context.Context) *logrus.Entry {
	entry := FromContext(ctx)
	return entry
}

// SetEntry updates the log entry in the context with the provided fields, and
// returns both. It is equivlent to:
//   entry := G(ctx).WithFields(fields)
//   ctx = WithContext(ctx, entry)
func SetEntry(ctx context.Context, fields logrus.Fields) (context.Context, *logrus.Entry) {
	e := G(ctx)
	if len(fields) > 0 {
		e = e.WithFields(fields)
	}
	ctx = WithContext(ctx, e)

	return ctx, e
}

// UpdateContext extracts the log entry from the context, and, if the entry's
// context points to a parent's of the current context, ands the entry
// to the most recent context.
//
// This allows the entry to reference the most recent context and any new
// values (such as span contexts) added to it.
func UpdateContext(ctx context.Context) context.Context {
	e := FromContext(ctx)
	if e.Context != ctx {
		ctx = WithContext(ctx, e)
	}

	return ctx
}

// WithContext returns a context that contains the provided log entry.
// The entry can be extracted with `G` (preferred) or `FromContext`.
//
// The entry in the context is a copy of `entry` (generated by `entry.WithContext`)
func WithContext(ctx context.Context, entry *logrus.Entry) context.Context {
	entry = entry.WithContext(ctx)

	return context.WithValue(ctx, _entryContextKey, entry)
}

// FromContext returns the log entry stored in the context, if one exits, or
// the default logging entry otherwise.
func FromContext(ctx context.Context) *logrus.Entry {
	entry := fromContext(ctx)

	if entry == nil {
		return L.WithContext(ctx)
	}

	return entry
}

// Copy extracts the tracing Span and logging entry from the src Context, if they
// exist, and adds them to the dst Context.
//
// This is useful to share tracing and logging between contexts, but not the
// cancellation. For example, if the src Context has been cancelled but cleanup
// operations triggered by the cancellation require a non-cancelled context to
// execute
func Copy(dst context.Context, src context.Context) context.Context {
	if s := trace.FromContext(src); s != nil {
		dst = trace.NewContext(dst, s)
	}

	if e := fromContext(src); e != nil {
		dst = WithContext(dst, e)
	}

	return dst
}

func fromContext(ctx context.Context) *logrus.Entry {
	e, _ := ctx.Value(_entryContextKey).(*logrus.Entry)

	return e
}

// IsLevelEnabled checks if the level of the logger stored in the context
// (or the default logger) is greather than the specified logging level.
func IsLevelEnabled(ctx context.Context, level logrus.Level) bool {
	l := L.Logger
	if e := fromContext(ctx); e != nil {
		l = e.Logger
	}

	return l.IsLevelEnabled(level)
}
