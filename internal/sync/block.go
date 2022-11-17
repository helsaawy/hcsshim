package sync

import (
	"context"
	"sync"
)

// Block represents a barrier for processes to wait on until a particular an event signals that
// further work can be safely unblocked.
type Block interface {
	// Wait waits until either the Block unblocks, allowing work to proceed, or the provided
	// [context.Context] is cancelled.
	// It returns either the error provided to [Close] or the Context's cancellation error.
	Wait(context.Context) error
	// Done returns a channel that's closed when when ErrBlock is unblocked.
	Done() <-chan struct{}
	// Close unblocks processes waiting on a Block, signalling that they may proceed.
	//
	// Not all Block instances can set their error on [Close].
	Close(error)
	// Closed returns true if the Block has been closed.
	Closed() bool
	// Err returns the error result of the blocking opteration, or nil, if the operation
	// was successfull or has not yet completed.
	//
	// Not all Block instances can set their error on [Close].
	Err() error
}

type errBlock struct {
	// once guards access to setting err and closing ch, to prevent double closing ch or
	// overwritting another thread setting err.
	once sync.Once
	ch   chan struct{}
	err  error
}

func NewErrorBlock() Block {
	return &errBlock{
		ch: make(chan struct{}),
	}
}

var _ Block = &errBlock{}

func (b *errBlock) Wait(ctx context.Context) error {
	select {
	case <-b.ch:
		return b.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *errBlock) Close(err error) {
	b.once.Do(func() {
		b.err = err
		close(b.ch)
	})
}

func (b *errBlock) Closed() bool {
	select {
	case <-b.ch:
		return true
	default:
		return false
	}
}

func (b *errBlock) Done() <-chan struct{} {
	return b.ch
}

func (b *errBlock) Err() error {
	return b.err
}
