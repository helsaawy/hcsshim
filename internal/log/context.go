package log

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"

	"github.com/Microsoft/hcsshim/internal/logfields"
)

type entryContextKeyType int

const _entryContextKey entryContextKeyType = iota

var (
	// L is the default, blank logging entry. WithField and co. all return a copy
	// of the original entry, so this will not leak fields between calls.
	//
	// Do NOT modify fields directly, as that will corrupt state for all users and
	// is not thread safe.
	// Instead, use `L.With*` or `L.Dup()`. Or `G(context.Background())`.
	L = logrus.NewEntry(logrus.StandardLogger())

	// G is an alias for GetEntry
	G = GetEntry

	// S is an alias for SetEntry
	S = SetEntry

	// U is an alias for UpdateContext
	U = UpdateContext
)

// todo: we only use the context to get span information, remove that from the hook and add it to G() and co.

// GetEntry returns a `logrus.Entry` stored in the context, if one exists.
// Otherwise, it returns a default entry that points to the current context.
//
// Note: if the a new entry is returned, it will reference the passed in context.
// However, existing contexts may be stored in parent contexts and additionally reference
// earlier contexts.
// Use `UpdateContext` to update the entry and context.
func GetEntry(ctx context.Context) *logrus.Entry {
	entry := fromContext(ctx)

	if entry == nil {
		entry = L.WithContext(ctx)
	}

	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		entry = entry.WithFields(logrus.Fields{
			logfields.TraceID: sc.TraceID().String(),
			logfields.SpanID:  sc.SpanID().String(),
		})
	}

	return entry
}

// SetEntry updates the log entry in the context with the provided fields, and
// returns both. It is equivalent to:
//
//	entry := GetEntry(ctx).WithFields(fields)
//	ctx = WithContext(ctx, entry)
//
// See WithContext for more information.
func SetEntry(ctx context.Context, fields logrus.Fields) (context.Context, *logrus.Entry) {
	e := GetEntry(ctx)
	if len(fields) > 0 {
		e = e.WithFields(fields)
	}
	return WithContext(ctx, e)
}

// UpdateContext extracts the log entry from the context, and, if the entry's
// context points to a parent's of the current context, ands the entry
// to the most recent context. It is equivalent to:
//
//	entry := GetEntry(ctx)
//	ctx = WithContext(ctx, entry)
//
// This allows the entry to reference the most recent context and any new
// values (such as span contexts) added to it.
//
// See WithContext for more information.
func UpdateContext(ctx context.Context) context.Context {
	// there is no way to check its ctx (and not one of its parents) that contains `e`
	// so, at a slight cost, force add `e` to the context
	ctx, _ = WithContext(ctx, GetEntry(ctx))
	return ctx
}

// WithContext returns a context that contains the provided log entry.
// The entry can be extracted with `GetEntry` (`G`)
//
// The entry in the context is a copy of `entry` (generated by `entry.WithContext`)
func WithContext(ctx context.Context, entry *logrus.Entry) (context.Context, *logrus.Entry) {
	// regardless of the order, entry.Context != GetEntry(ctx)
	// here, the returned entry will reference the supplied context
	entry = entry.WithContext(ctx)
	ctx = context.WithValue(ctx, _entryContextKey, entry)

	return ctx, entry
}

// Copy extracts the tracing Span and logging entry from the src Context, if they
// exist, and adds them to the dst Context.
//
// This is useful to share tracing and logging between contexts, but not the
// cancellation. For example, if the src Context has been cancelled but cleanup
// operations triggered by the cancellation require a non-cancelled context to
// execute.
func Copy(dst context.Context, src context.Context) context.Context {
	if sc := trace.SpanContextFromContext(src); sc.IsValid() {
		dst = trace.ContextWithSpanContext(src, sc)
	}

	if e := fromContext(src); e != nil {
		dst, _ = WithContext(dst, e)
	}

	return dst
}

func fromContext(ctx context.Context) *logrus.Entry {
	e, _ := ctx.Value(_entryContextKey).(*logrus.Entry)
	return e
}
