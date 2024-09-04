package retry

import (
	"context"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/stretchr/testify/require"
	"io"
	"testing"
	"time"
)

// TestWithBackoff_Trivial tests a static function returning a non-error.
func TestWithBackoff_Trivial(t *testing.T) {
	require.NoError(t,
		WithBackoff(
			context.Background(),
			func(_ context.Context) error { return nil },
			func(_ error) bool { return false },
			func(_ uint64) time.Duration { return 0 },
			Settings{}))
}

// TestWithBackoff_NotRetryable tests a static function retuning an error, marked as non-retryable.
func TestWithBackoff_NotRetryable(t *testing.T) {
	err := WithBackoff(
		context.Background(),
		func(_ context.Context) error { return io.EOF },
		func(_ error) bool { return false },
		func(_ uint64) time.Duration { return 0 },
		Settings{})

	require.ErrorAs(t, err, &io.EOF)
	require.ErrorContains(t, err, "can't retry")
}

// TestWithBackoff_Panic tests a panicking function, expecting to receive the panic.
func TestWithBackoff_Panic(t *testing.T) {
	require.Panics(t, func() {
		_ = WithBackoff(
			context.Background(),
			func(_ context.Context) error { panic(":<") },
			func(_ error) bool { return false },
			func(_ uint64) time.Duration { return 0 },
			Settings{})
	})
}

// TestWithBackoff_SimpleRetry tests retrying a function which returns a retryable error only the first time.
func TestWithBackoff_SimpleRetry(t *testing.T) {
	isReady := false

	require.NoError(t,
		WithBackoff(
			context.Background(),
			func(_ context.Context) error {
				if !isReady {
					isReady = true
					return io.EOF
				}
				return nil
			},
			Retryable,
			func(_ uint64) time.Duration { return 0 },
			Settings{}))
}

// TestWithBackoff_ContextDone tests a static function returning a retryable error until the context has timed out.
func TestWithBackoff_ContextDone(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	require.ErrorAs(t,
		WithBackoff(
			ctx,
			func(ctx context.Context) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return io.EOF
			},
			Retryable,
			backoff.NewExponentialWithJitter(time.Millisecond, 10*time.Millisecond),
			Settings{}),
		&context.DeadlineExceeded)
}

// TestWithBackoff_ContextDoneBlockingFunc tests a static function returning a retryable error after sleeping a bit
// until the context has timed out. As the backoff has no delay, all blocking is performed in the RetryableFunc,
// resulting to exit in the context error check after the function call and not in the final select.
func TestWithBackoff_ContextDoneBlockingFunc(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	require.ErrorAs(t,
		WithBackoff(
			ctx,
			func(ctx context.Context) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				time.Sleep(10 * time.Millisecond)
				return io.EOF
			},
			Retryable,
			func(_ uint64) time.Duration { return 0 },
			Settings{}),
		&context.DeadlineExceeded)
}

// TestWithBackoff_TimeoutEventuallyOk tests a function returning a non-error after being called elven times while using
// a Settings.Timeout.
func TestWithBackoff_TimeoutEventuallyOk(t *testing.T) {
	readyCountdown := 10

	require.NoError(t,
		WithBackoff(
			context.Background(),
			func(_ context.Context) error {
				if readyCountdown > 0 {
					readyCountdown--
					return io.EOF
				}
				return nil
			},
			Retryable,
			backoff.NewExponentialWithJitter(time.Millisecond, 10*time.Millisecond),
			Settings{Timeout: 500 * time.Millisecond}))
}

// TestWithBackoff_TimeoutFail tests a static function returning an error while using a Settings.Timeout, expecting to
// eventually hit this timeout.
func TestWithBackoff_TimeoutFail(t *testing.T) {
	err := WithBackoff(
		context.Background(),
		func(_ context.Context) error { return io.EOF },
		Retryable,
		backoff.NewExponentialWithJitter(time.Millisecond, 10*time.Millisecond),
		Settings{Timeout: 500 * time.Millisecond})

	require.ErrorAs(t, err, &io.EOF)
	require.ErrorContains(t, err, "retry deadline exceeded")
}

// TestWithBackoff_TimeoutBlockingFunc tests a static function returning an error after blocking for quite some time
// while using a Settings.Timeout and having a zero backoff duration. Compared to the previous test,
// TestWithBackoff_TimeoutFail, this will not result in a re-run of the RetryableFunc.
func TestWithBackoff_TimeoutBlockingFunc(t *testing.T) {
	err := WithBackoff(
		context.Background(),
		func(_ context.Context) error {
			time.Sleep(10 * time.Millisecond)
			return io.EOF
		},
		Retryable,
		func(_ uint64) time.Duration { return 0 },
		Settings{Timeout: 500 * time.Millisecond})

	require.ErrorAs(t, err, &io.EOF)
	require.ErrorContains(t, err, "retry deadline exceeded")
}

// TestWithBackoff_Callback tests a function returning a non-error after being called elven times while having both a
// Settings.OnRetryableError and a Settings.OnSuccess defined.
func TestWithBackoff_Callback(t *testing.T) {
	readyCountdown := 10
	errorCallbackCounter := uint64(0)
	successCallbackCounter := uint64(0)

	require.NoError(t,
		WithBackoff(
			context.Background(),
			func(_ context.Context) error {
				if readyCountdown > 0 {
					readyCountdown--
					return io.EOF
				}
				return nil
			},
			Retryable,
			func(_ uint64) time.Duration { return 0 },
			Settings{
				OnRetryableError: func(_ time.Duration, c uint64, _, _ error) { errorCallbackCounter = c },
				OnSuccess:        func(_ time.Duration, c uint64, _ error) { successCallbackCounter = c },
			}))

	require.Equal(t, uint64(10), errorCallbackCounter, "last OnRetryableError attempt")
	require.Equal(t, uint64(11), successCallbackCounter, "OnSuccess attempt")
}

// TestWithBackoff_QuickContextExit tests retrying a function which returns a retryable error only the first time while
// having Settings.QuickContextExit defined. However, the backoff is a magnitude smaller than the context timeout.
func TestWithBackoff_QuickContextExit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	isReady := false

	require.NoError(t,
		WithBackoff(
			ctx,
			func(ctx context.Context) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}

				if !isReady {
					isReady = true
					return io.EOF
				}
				return nil
			},
			Retryable,
			backoff.NewExponentialWithJitter(time.Millisecond, 10*time.Millisecond),
			Settings{QuickContextExit: true}))
}

// TestWithBackoff_QuickContextExitPanic tests a panicking function while having Settings.QuickContextExit defined. It
// is expected to be recovered and returned as an error.
func TestWithBackoff_QuickContextExitPanic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	require.ErrorContains(t,
		WithBackoff(
			ctx,
			func(_ context.Context) error { panic(":<") },
			Retryable,
			backoff.NewExponentialWithJitter(time.Millisecond, 10*time.Millisecond),
			Settings{QuickContextExit: true}),
		"retryable function panicked, :<")
}

// TestWithBackoff_QuickContextExitTimeout tests a static function returning no error after blocking for an eternity
// while having Settings.QuickContextExit defined. The context exceeds ages before the RetryableFunc.
func TestWithBackoff_QuickContextExitTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	errCh := make(chan error)
	go func() {
		errCh <- WithBackoff(
			ctx,
			func(_ context.Context) error {
				time.Sleep(time.Second)
				return nil
			},
			Retryable,
			func(_ uint64) time.Duration { return 0 },
			Settings{QuickContextExit: true})
		close(errCh)
	}()

	select {
	case err := <-errCh:
		require.ErrorAs(t, err, &context.DeadlineExceeded)
	case <-time.After(500 * time.Millisecond):
		require.Fail(t, "timeout, context is long gone")
	}
}
