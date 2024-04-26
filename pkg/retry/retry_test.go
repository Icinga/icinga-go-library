package retry

import (
	"context"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"
)

func TestWithBackoff(t *testing.T) {
	transitionErr := errors.New("transition")
	retryableErr := errors.New("retryable")
	permanentErr := errors.New("permanent")
	isRetryable := func(err error) bool {
		return errors.Is(err, transitionErr) || errors.Is(err, retryableErr)
	}
	noSleep := func(_ uint64) time.Duration {
		return 0
	}

	t.Run("Success on first attempt", func(t *testing.T) {
		attempt := 0
		err := WithBackoff(context.Background(), func(ctx context.Context) error {
			attempt++

			return nil
		}, isRetryable, noSleep, Settings{})
		require.Equal(t, 1, attempt)
		require.NoError(t, err)
	})

	t.Run("Success on third attempt", func(t *testing.T) {
		attempt := 0
		err := WithBackoff(context.Background(), func(ctx context.Context) error {
			attempt++

			if attempt < 2 {
				return transitionErr
			}

			if attempt < 3 {
				return retryableErr
			}

			return nil
		}, isRetryable, noSleep, Settings{})
		require.Equal(t, 3, attempt)
		require.NoError(t, err)
	})

	t.Run("Fail if not retryable", func(t *testing.T) {
		attempt := 0
		err := WithBackoff(context.Background(), func(ctx context.Context) error {
			attempt++

			return permanentErr
		}, isRetryable, noSleep, Settings{})
		require.Equal(t, 1, attempt)
		if !errors.Is(err, permanentErr) {
			t.Errorf("Expected error %v, got %v", permanentErr, err)
		}
	})

	t.Run("Fail if no longer retryable", func(t *testing.T) {
		attempt := 0
		err := WithBackoff(context.Background(), func(ctx context.Context) error {
			attempt++

			if attempt < 2 {
				return transitionErr
			}

			if attempt < 3 {
				return retryableErr
			}

			return permanentErr
		}, isRetryable, noSleep, Settings{})
		require.Equal(t, 3, attempt)
		if !errors.Is(err, permanentErr) {
			t.Errorf("Expected error %v, got %v", permanentErr, err)
		}
	})

	t.Run("Context cancelling", func(t *testing.T) {
		ctx, cancelCtx := context.WithCancel(context.Background())
		defer cancelCtx()

		attempt := 0
		err := WithBackoff(ctx, func(ctx context.Context) error {
			attempt++

			if attempt == 2 {
				cancelCtx()

				// TODO(el): Account return nil and return err other than context.Canceled.
				return context.Canceled
			}

			return retryableErr
		}, isRetryable, noSleep, Settings{})
		require.Equal(t, 2, attempt)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected error %v, got %v", context.Canceled, err)
		}
	})

	t.Run("Context canceled", func(t *testing.T) {
		ctx, cancelCtx := context.WithCancel(context.Background())
		cancelCtx()

		attempt := 0
		err := WithBackoff(ctx, func(ctx context.Context) error {
			attempt++

			return retryableErr
		}, isRetryable, noSleep, Settings{})
		require.Zero(t, attempt, "Expected retryable function is not executed if context already canceled")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected error %v, got %v", context.Canceled, err)
		}
	})

	t.Run("Timeout", func(t *testing.T) {
		timeout := 100 * time.Millisecond

		var attempt atomic.Int64
		errs := make(chan error, 1)
		go func() {
			defer close(errs)

			err := WithBackoff(context.Background(), func(ctx context.Context) error {
				attempt.Add(1)

				return retryableErr
			}, isRetryable, noSleep, Settings{Timeout: timeout})

			errs <- err
		}()

		select {
		case err := <-errs:
			require.Error(t, err)
			require.NotZero(t, attempt.Load())
		case <-time.After(2 * timeout):
			t.Error("Timeout waiting for timeout")
		}
	})

	t.Run("Retryable function exceeds timeout", func(t *testing.T) {
		timeout := 100 * time.Millisecond

		var attempt atomic.Int64
		errs := make(chan error, 1)
		go func() {
			defer close(errs)

			err := WithBackoff(context.Background(), func(ctx context.Context) error {
				if attempt.CompareAndSwap(0, 1) {
					time.Sleep(timeout)
				} else {
					attempt.Add(1)
				}

				return retryableErr
			}, isRetryable, noSleep, Settings{Timeout: timeout})

			errs <- err
		}()

		select {
		case err := <-errs:
			require.Error(t, err)
			require.EqualValues(t, 1, attempt.Load())
		case <-time.After(2 * timeout):
			t.Error("Timeout waiting for timeout")
		}
	})

	t.Run("Timeout initiates final attempt", func(t *testing.T) {
		timeout := 100 * time.Millisecond

		var attempt atomic.Int64
		errs := make(chan error, 1)
		go func() {
			defer close(errs)

			err := WithBackoff(context.Background(), func(ctx context.Context) error {
				attempt.Add(1)

				return retryableErr
			}, isRetryable, func(u uint64) time.Duration {
				return 2 * timeout
			}, Settings{Timeout: timeout})

			errs <- err
		}()

		select {
		case err := <-errs:
			require.Error(t, err)
			require.EqualValues(t, 2, attempt.Load())
		case <-time.After(2 * timeout):
			t.Error("Timeout waiting for timeout")
		}
	})

	t.Run("OnSuccess", func(t *testing.T) {
		type onSuccess struct {
			attempt uint64
			lastErr error
		}
		var success *onSuccess
		attempt := 0

		err := WithBackoff(context.Background(), func(ctx context.Context) error {
			attempt++

			if attempt < 2 {
				return transitionErr
			}

			if attempt == 3 {
				return nil
			}

			return retryableErr
		}, isRetryable, noSleep, Settings{OnSuccess: func(_ time.Duration, attempt uint64, lastErr error) {
			success = &onSuccess{
				attempt: attempt,
				lastErr: lastErr,
			}
		}})

		require.NotNil(t, success, "Expected OnSuccess callback to be called")
		require.NoError(t, err)
		require.Equal(t, 3, attempt)
		require.EqualValues(t, attempt, success.attempt)
		if !errors.Is(success.lastErr, retryableErr) {
			t.Errorf("Expected error %v, got %v", retryableErr, err)
		}
	})

	t.Run("OnRetryableError", func(t *testing.T) {
		type onRetryableError struct {
			attempt uint64
			err     error
			lastErr error
		}
		attempt := 0
		retries := make(chan onRetryableError, 2)

		err := WithBackoff(context.Background(), func(ctx context.Context) error {
			attempt++

			if attempt < 2 {
				return transitionErr
			}

			if attempt == 3 {
				return permanentErr
			}

			return retryableErr
		}, isRetryable, noSleep, Settings{OnRetryableError: func(_ time.Duration, attempt uint64, err, lastErr error) {
			select {
			case retries <- onRetryableError{
				attempt: attempt,
				err:     err,
				lastErr: lastErr,
			}:
			default:
				t.Errorf("OnRetryableError() was called unexpectedly for attempt %v", attempt)
			}
		}})
		close(retries)

		if !errors.Is(err, permanentErr) {
			t.Errorf("Expected error %v, got %v", permanentErr, err)
		}
		require.Equal(t, 3, attempt)

		expected := []onRetryableError{
			{
				attempt: 1,
				err:     transitionErr,
				lastErr: nil,
			},
			{
				attempt: 2,
				err:     retryableErr,
				lastErr: transitionErr,
			},
		}

		var result []onRetryableError
		for retryableError := range retries {
			result = append(result, retryableError)
		}
		require.Equal(t, expected, result)
	})
}

func TestResetTimeout(t *testing.T) {
	t.Run("Expired timer", func(t *testing.T) {
		timer := time.NewTimer(time.Millisecond)
		<-timer.C
		ResetTimeout(timer, 100*time.Millisecond)

		select {
		case <-timer.C:
		case <-time.After(200 * time.Millisecond):
			t.Error("Timer did not expire after resetting")
		}
	})

	t.Run("Active timer", func(t *testing.T) {
		activeTimer := time.NewTimer(100 * time.Millisecond)
		ResetTimeout(activeTimer, 500*time.Millisecond)

		select {
		case <-activeTimer.C:
			t.Error("Active timer expired after resetting")
		case <-time.After(200 * time.Millisecond):
		}
	})

	t.Run("Stopped timer", func(t *testing.T) {
		stoppedTimer := time.NewTimer(100 * time.Millisecond)
		assert.True(t, stoppedTimer.Stop())
		ResetTimeout(stoppedTimer, 500*time.Millisecond)

		select {
		case <-stoppedTimer.C:
			t.Error("Stopped timer expired after resetting")
		case <-time.After(200 * time.Millisecond):
		}
	})

	t.Run("Non-expired timer", func(t *testing.T) {
		timer := time.NewTimer(200 * time.Millisecond)
		ResetTimeout(timer, 100*time.Millisecond)

		select {
		case <-timer.C:
		case <-time.After(500 * time.Millisecond):
			t.Error("Timer did not expire after resetting")
		}
	})
}
