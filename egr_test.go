package egr_test

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/invisiblefunnel/egr"
)

// TestWithContext replicates errgroup_test’s approach: once a goroutine
// returns an error, the group's context should be canceled, and Wait
// should return that error.
func TestWithContext(t *testing.T) {
	errDoom := errors.New("group_test: doomed")

	type testCase struct {
		errs []error
		want error
	}

	cases := []testCase{
		{errs: []error{}, want: nil},
		{errs: []error{nil}, want: nil},
		{errs: []error{errDoom}, want: errDoom},
		{errs: []error{errDoom, nil}, want: errDoom},
		{errs: []error{nil, errDoom}, want: errDoom},
	}

	for _, tc := range cases {
		ctx := context.Background()
		g, ctx := egr.WithContext[int](ctx, 2)

		for _, e := range tc.errs {
			e := e // capture
			g.Go(func(_ <-chan int) error { return e })
		}

		got := g.Wait()
		if got != tc.want {
			t.Errorf("For errs=%v, Wait() = %v; want %v", tc.errs, got, tc.want)
		}

		// The group’s returned context should be canceled once any error is encountered
		select {
		case <-ctx.Done():
			// ctx is canceled
		default:
			// If we expected an error (non-nil) but the context isn't canceled, that's a bug
			if tc.want != nil {
				t.Errorf("Context was not canceled but expected an error %v", tc.want)
			}
		}
	}
}

func TestPushContextDone(t *testing.T) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		100*time.Millisecond,
	)
	defer cancel()

	g, ctx := egr.WithContext[int](ctx, 1)

	for i := 0; i < 5; i++ {
		g.Go(func(queue <-chan int) error {
			for range queue {
			}
			return nil
		})
	}

	// Loop until the context deadline is exceeded
	for {
		if err := g.Push(ctx, 0); err != nil {
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("expected '%v' error return from Push, got %v", context.DeadlineExceeded, err)
			}
			break
		}
	}

	err := g.Wait()
	if err != nil {
		t.Errorf("unexpected error return from Wait: %v", err)
	}
}

func TestGoPushWait(t *testing.T) {
	ctx := context.Background()
	g, ctx := egr.WithContext[int](ctx, 2)

	var (
		consumed []int
		lock     sync.Mutex
	)

	nRoutines := 5
	for i := 0; i < nRoutines; i++ {
		g.Go(func(queue <-chan int) error {
			for item := range queue {
				lock.Lock()
				consumed = append(consumed, item)
				lock.Unlock()
			}
			return nil
		})
	}

	n := 1000
	for i := 0; i < n; i++ {
		err := g.Push(ctx, i)
		if err != nil {
			t.Errorf("unexpected error return from Push: %v", err)
		}
	}

	err := g.Wait()
	if err != nil {
		t.Errorf("unexpected error return from Wait: %v", err)
	}

	if len(consumed) != n {
		t.Errorf("expected %d items consumed, got %d", n, len(consumed))
	}

	sort.Ints(consumed)
	for i := range consumed {
		if i != consumed[i] {
			t.Errorf("expected consumed item %d, got %d", i, consumed[i])
		}
	}
}

// BenchmarkGo measures overhead of spawning goroutines in egr.Group.
func BenchmarkGo(b *testing.B) {
	ctx := context.Background()
	fn := func(_ <-chan int) error { return nil }

	b.ResetTimer()
	b.ReportAllocs()

	// We create a new group once, spawn b.N goroutines, then Wait.
	// This is slightly different from the original which tested repeated spawns,
	// but it mirrors the general overhead test for egr.
	for i := 0; i < b.N; i++ {
		// Each iteration of b.N spawns one goroutine
		g, _ := egr.WithContext[int](ctx, 0)
		g.Go(fn)
		g.Wait()
	}
}