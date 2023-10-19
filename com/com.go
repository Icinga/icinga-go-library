package com

import (
	"context"
	"github.com/icinga/icinga-go-library/types"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// WaitAsync calls Wait() on the passed Waiter in a new goroutine and
// sends the first non-nil error (if any) to the returned channel.
// The returned channel is always closed when the Waiter is done.
func WaitAsync(ctx context.Context, w Waiter) <-chan error {
	errs := make(chan error, 1)

	go func() {
		defer close(errs)

		if e := w.Wait(); e != nil {
			select {
			case errs <- e:
			case <-ctx.Done():
			}
		}
	}()

	return errs
}

// ErrgroupReceive adds a goroutine to the specified group that
// returns the first non-nil error (if any) from the specified channel.
// If the channel is closed, it will return nil.
func ErrgroupReceive(ctx context.Context, g *errgroup.Group, err <-chan error) {
	g.Go(func() error {
		select {
		case e, more := <-err:
			if !more {
				return nil
			}

			return e
		case <-ctx.Done():
			return ctx.Err()
		}
	})
}

// CopyFirst asynchronously forwards all items from input to forward and synchronously returns the first item.
func CopyFirst[T any](ctx context.Context, input <-chan T) (T, <-chan T, error) {
	select {
	case first, ok := <-input:
		if !ok {
			return types.Zero[T](), nil, errors.New("can't read from closed channel")
		}

		// Buffer of one because we receive an entity and send it back immediately.
		forward := make(chan T, 1)
		forward <- first

		go func() {
			defer close(forward)

			for {
				select {
				case e, ok := <-input:
					if !ok {
						return
					}

					select {
					case forward <- e:
					case <-ctx.Done():
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()

		return first, forward, nil
	case <-ctx.Done():
		return types.Zero[T](), nil, ctx.Err()
	}
}
