package periodic

import (
	"context"
	"time"
)

// WindUpContext wraps a context.Context with a wind-up timeout, allowing to be extended multiple times.
//
// After calling WindUpContext, the returned Context behaves like being created via context.WithTimeout. However, each
// time the returned function will be called with a new timeout, the Context's lifetime will be set to this value. When
// the timeout has exceeded, the Context will finish and cannot be winded-up again.
//
// Thus, the Context can be wind-up similar to a wind-up clock or wind-up toy. Besides this analogy, think about a
// moving deadline or moving timeout.
//
// The wind-up function returns an error if the internal Context has finished.
func WindUpContext(parent context.Context, timeout time.Duration) (context.Context, func(time.Duration) error) {
	ctx, cancel := context.WithCancel(parent)

	windUpChan := make(chan time.Duration)
	windUpFn := func(timeout time.Duration) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		windUpChan <- timeout
		return nil
	}

	go func() {
		timer := time.NewTimer(timeout)

		defer func() {
			_ = timer.Stop()
			cancel()
			close(windUpChan)
		}()

		for {
			select {
			case <-parent.Done():
				return
			case <-timer.C:
				return
			case newTimeout := <-windUpChan:
				_ = timer.Reset(newTimeout)
			}
		}
	}()

	return ctx, windUpFn
}
