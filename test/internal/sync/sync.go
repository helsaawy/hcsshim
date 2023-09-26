package sync

import (
	"context"
	"sync"
)

type OnceFunc[T any] func(context.Context) (T, error)

// Once is a wrapper around [sync.Once] that enforces a type constraint on the return value,
// accepts a context at call time, and also stores an error separate from the return value.
type Once[T any] struct {
	o   sync.Once
	f   OnceFunc[T]
	v   T
	err error
}

func NewOnce[T any](f OnceFunc[T]) *Once[T] {
	return &Once[T]{f: f}
}

// Value is similar to [Do], but discards the error value.
func (o *Once[T]) Value(ctx context.Context) T {
	v, _ := o.Do(ctx)
	return v
}

// Do calls the [OnceFunc] with the suplied [context.Context] exactly once and returns the
// result.
//
// It is analogous to [sync.Do].
func (o *Once[T]) Do(ctx context.Context) (T, error) {
	if o == nil || o.f == nil {
		panic("uninitialized Once")
	}

	o.o.Do(func() {
		o.v, o.err = o.f(ctx)
	})
	return o.v, o.err
}
