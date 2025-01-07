package egr

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// Group[T] is a collection of goroutines processing
// items of type T from a shared queue.
type Group[T any] struct {
	group *errgroup.Group
	queue chan T
}

// WithContext returns a new Group[T] along with a derived context.Context.
// The group's goroutines will be canceled if any goroutine returns a non-nil error.
func WithContext[T any](ctx context.Context, queueSize int) (*Group[T], context.Context) {
	group, ctx := errgroup.WithContext(ctx)
	queue := make(chan T, queueSize)
	return &Group[T]{group, queue}, ctx
}

// SetLimit limits the number of active goroutines in this group to at most n.
// A negative value indicates no limit. Any subsequent call to the Go method will
// block until it can add an active goroutine without exceeding the configured limit.
// The limit must not be modified while any goroutines in the group are active.
func (g *Group[T]) SetLimit(n int) {
	g.group.SetLimit(n)
}

// TryGo calls the given function in a new goroutine only if the number of
// active goroutines in the group is currently below the configured limit.
// The return value reports whether the goroutine was started.
func (g *Group[T]) TryGo(f func(queue <-chan T) error) bool {
	return g.group.TryGo(func() error {
		return f(g.queue)
	})
}

// Go runs a function in a new goroutine, passing a read-only channel of type T.
// If any goroutine returns an error, the context is canceled and the error is propagated.
func (g *Group[T]) Go(f func(queue <-chan T) error) {
	g.group.Go(func() error {
		return f(g.queue)
	})
}

// Push sends an item of type T into the queue.
// If the provided ctx is canceled, Push returns the context's error.
// Push must not be called after Wait.
func (g *Group[T]) Push(ctx context.Context, item T) error {
	select {
	case g.queue <- item:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Wait closes the queue channel and waits for all goroutines to complete,
// returning the first error encountered (if any).
func (g *Group[T]) Wait() error {
	close(g.queue)
	return g.group.Wait()
}
