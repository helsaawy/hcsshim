package sync

import (
	"context"
	"sync"
)

// TODO (go1.21): use pkg.go.dev/sync#OnceValues

// OnceValue is a wrapper around [sync.Once] that runs f only once and
// returns both a value (of type T) and an error.
func OnceValue[T any](f func() (T, error)) func() (T, error) {
	once := &OnceErr[T]{}

	return func() (T, error) {
		return once.Do(f)
	}
}

// NewOnce is similar to [OnceValue], but allows passing a context to f.
func OnceValueCtx[T any](f func(context.Context) (T, error)) func(context.Context) (T, error) {
	once := &OnceErr[T]{}

	return func(ctx context.Context) (T, error) {
		return once.DoCtx(ctx, f)
	}
}

type OnceErr[T any] struct {
	once sync.Once
	v    T
	err  error
}

func (o *OnceErr[T]) Do(f func() (T, error)) (T, error) {
	o.once.Do(func() {
		o.v, o.err = f()
	})
	return o.v, o.err
}

func (o *OnceErr[T]) DoCtx(ctx context.Context, f func(context.Context) (T, error)) (T, error) {
	o.once.Do(func() {
		o.v, o.err = f(ctx)
	})
	return o.v, o.err
}
