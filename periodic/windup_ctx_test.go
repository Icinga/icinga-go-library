package periodic

import (
	"context"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func TestWindUpContext(t *testing.T) {
	// timeout to be used for both the initial duration and for winding ups.
	// When being checked against, a delta of 10% is added against races.
	const timeout = 100 * time.Millisecond
	const timeoutDelta = timeout + timeout/10

	// requireNoTimeout waits for timeoutDelta and errors if the context has finished.
	requireNoTimeout := func(t *testing.T, ctx context.Context) {
		select {
		case <-ctx.Done():
			require.Fail(t, "context timed out")
		case <-time.After(timeoutDelta):
		}
	}

	// requireTimeout waits for timeoutDelta and errors if the context has not finished.
	requireTimeout := func(t *testing.T, ctx context.Context) {
		select {
		case <-ctx.Done():
		case <-time.After(timeoutDelta):
			require.Fail(t, "context did not timed out")
		}
	}

	t.Run("timeout", func(t *testing.T) {
		ctx, _ := WindUpContext(context.Background(), timeout)
		requireTimeout(t, ctx)
	})

	t.Run("wind-up-once", func(t *testing.T) {
		ctx, windUpFn := WindUpContext(context.Background(), timeout)

		_ = time.AfterFunc(timeout/2, func() { require.NoError(t, windUpFn(timeout)) })

		requireNoTimeout(t, ctx)
		requireTimeout(t, ctx)
	})

	t.Run("wind-up-multiple", func(t *testing.T) {
		ctx, windUpFn := WindUpContext(context.Background(), timeout)

		for i := 0; i < 5; i++ {
			// Two times as requireNoTimeout adds the delta,
			// but total time of requireNoTimeout and requireTimeout > 2*timeout due to two deltas.
			require.NoError(t, windUpFn(2*timeout))
			requireNoTimeout(t, ctx)
		}

		requireTimeout(t, ctx)
	})

	t.Run("wind-up-rewind", func(t *testing.T) {
		ctx, windUpFn := WindUpContext(context.Background(), timeout)

		// Wind up after timeout/3, 2*timeout/3 and timeout.
		for i := 1; i <= 3; i++ {
			_ = time.AfterFunc((time.Duration(i)*timeout)/3, func() { require.NoError(t, windUpFn(timeout)) })
		}

		requireNoTimeout(t, ctx)
		requireTimeout(t, ctx)
	})

	t.Run("wind-up-parallel-flood", func(t *testing.T) {
		ctx, windUpFn := WindUpContext(context.Background(), timeout)

		var wg sync.WaitGroup
		wg.Add(1_000) // https://100go.co/#misusing-syncwaitgroup-71
		for i := 0; i < 1_000; i++ {
			go func() {
				require.NoError(t, windUpFn(timeout))
				wg.Done()
			}()
		}
		wg.Wait()

		requireTimeout(t, ctx)
	})

	t.Run("wind-up-expired", func(t *testing.T) {
		ctx, windUpFn := WindUpContext(context.Background(), timeout)

		requireTimeout(t, ctx)

		require.Error(t, windUpFn(timeout))
	})

	t.Run("parent-done", func(t *testing.T) {
		parent, cancel := context.WithCancel(context.Background())
		ctx, _ := WindUpContext(parent, timeout)

		_ = time.AfterFunc(timeout/5, cancel)

		// Cannot use require{No,}Timeout here, as dealing with a fraction of the timeout
		select {
		case <-ctx.Done():
		case <-time.After(timeout / 2):
			t.Error("parent was already canceled")
		}
	})
}
