package retry

import (
	"context"
	"database/sql/driver"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"io"
	"net"
	"syscall"
	"time"
)

// DefaultTimeout is our opinionated default timeout for retrying database and Redis operations.
const DefaultTimeout = 5 * time.Minute

// RetryableFunc is a retryable function.
type RetryableFunc func(context.Context) error

// IsRetryable checks whether a new attempt can be started based on the error passed.
type IsRetryable func(error) bool

// OnRetryableErrorFunc is called if a retryable error occurs.
type OnRetryableErrorFunc func(elapsed time.Duration, attempt uint64, err, lastErr error)

// OnSuccessFunc is called once the operation succeeds.
type OnSuccessFunc func(elapsed time.Duration, attempt uint64, lastErr error)

// Settings aggregates optional settings for WithBackoff.
type Settings struct {
	// Timeout, if > 0, lets WithBackoff stop retrying gracefully once elapsed based on the following criteria:
	//
	// 	* If the execution of RetryableFunc has taken longer than Timeout, no further attempts are made.
	// 	* If Timeout elapses during the sleep phase between retries, one final retry is attempted.
	// 	* RetryableFunc is always granted its full execution time and is not canceled if it exceeds Timeout unless
	//	  QuickContextExit is set.
	//
	// This means that WithBackoff may not stop exactly after Timeout expires,
	// or may not retry at all if the first execution of RetryableFunc already takes longer than Timeout.
	Timeout time.Duration

	// OnRetryableError, if not nil, is called if a retryable error occurred.
	OnRetryableError OnRetryableErrorFunc

	// OnSuccess, if not nil, is called after the function succeeded.
	OnSuccess OnSuccessFunc

	// QuickContextExit, if set, directly aborts if the context is done and does not wait for functions to finish.
	//
	// Technically, all potentially blocking functions - the passed RetryableFunc as well as OnRetryableError and
	// OnSuccess, if set - are then being executed in another Goroutine via contextBoundFunc. The moment the context
	// expires, an error is returned while the function continues in its Goroutine, while its return value will be
	// discarded.
	QuickContextExit bool
}

// contextBoundFunc wraps an error generating function, but directly exits with an error if the context is done.
//
// While the function parameter signature matches RetryableFunc, this is not used here as this function does not only
// focuses on RetryableFunc, but more generally functions which might get aborted within WithBackoff.
func contextBoundFunc(f func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		fResultCh := make(chan error, 1)

		go func() {
			var err error

			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("retryable function panicked, %s", r)
				}

				fResultCh <- err
			}()

			err = f(ctx)
		}()

		select {
		case err := <-fResultCh:
			return err

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// WithBackoff retries the retryableFunc in case of failure.
//
// The passed ctx loosely restricts retries and prohibits retries if the context is done. It is also being passed to the
// retryableFunc, which MUST honor the context as well. However, unless Settings.QuickContextExit is set, the
// RetryableFunc blocks the return of the WithBackoff function.
//
// If retryableFunc has returned an error, retryable decides based on this error if another attempt should be made. This
// package comes with the Retryable() function collecting common errors considered recoverable.
//
// For debouncing, a delay between a failed run and making another attempt can be defined by backoffFn.
//
// Additional configuration and tweaks can be set via settings, as further documented at the Settings type.
func WithBackoff(
	ctx context.Context,
	retryableFunc RetryableFunc,
	retryable IsRetryable,
	backoffFn backoff.Backoff,
	settings Settings,
) (err error) {
	// Channel for retry deadline, which is set to the channel of NewTimer() if a timeout is configured,
	// otherwise nil, so that it blocks forever if there is no timeout.
	var timeout <-chan time.Time
	if settings.Timeout > 0 {
		t := time.NewTimer(settings.Timeout)
		defer t.Stop()
		timeout = t.C
	}

	// funcWrapper is wrapped around potentially time-consuming blocks: the RetryableFunc and, if configured, the two
	// callback functions. By default, funcWrapper is just a path-through identify function. However, with
	// Settings.QuickContextExit set, it will be contextBoundFunc, directly bailing out when the context is done.
	var funcWrapper = func(f func(context.Context) error) func(context.Context) error { return f }
	if settings.QuickContextExit {
		funcWrapper = contextBoundFunc
	}

	start := time.Now()
	timedOut := false
	for attempt := uint64(1); ; attempt++ {
		prevErr := err

		err = funcWrapper(func(ctx context.Context) error {
			err := retryableFunc(ctx)
			if err == nil {
				if settings.OnSuccess != nil {
					settings.OnSuccess(time.Since(start), attempt, prevErr)
				}
			}
			return err
		})(ctx)
		if err == nil {
			return
		}

		// Retryable function may have exited prematurely due to context errors.
		// We explicitly check the context error here, as the error returned by the retryable function can pass the
		// error.Is() checks even though it is not a real context error, for example:
		// https://cs.opensource.google/go/go/+/refs/tags/go1.22.2:src/net/net.go;l=422
		// https://cs.opensource.google/go/go/+/refs/tags/go1.22.2:src/net/net.go;l=601
		if ctx.Err() != nil {
			err = ctx.Err()
			if prevErr != nil {
				err = errors.Wrap(err, prevErr.Error())
			}
			return
		}

		if !retryable(err) {
			err = errors.Wrap(err, "can't retry")
			return
		}

		select {
		case <-timeout:
			// Stop retrying immediately if executing the retryable function took longer than the timeout.
			timedOut = true
		default:
		}

		if timedOut {
			err = errors.Wrap(err, "retry deadline exceeded")
			return
		}

		if settings.OnRetryableError != nil {
			_ = funcWrapper(func(_ context.Context) error {
				settings.OnRetryableError(time.Since(start), attempt, err, prevErr)
				return nil
			})(ctx)
		}

		select {
		case <-time.After(backoffFn(attempt)):
		case <-timeout:
			// Do not stop retrying immediately, but start one last attempt to mitigate timing issues where
			// the timeout expires while waiting for the next attempt and
			// therefore no retries have happened during this possibly long period.
			timedOut = true
		case <-ctx.Done():
			err = errors.Wrap(ctx.Err(), err.Error())
			return
		}
	}
}

// ResetTimeout changes the possibly expired timer t to expire after duration d.
//
// If the timer has already expired and nothing has been received from its channel,
// it is automatically drained as if the timer had never expired.
func ResetTimeout(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		<-t.C
	}

	t.Reset(d)
}

// Retryable returns true for common errors that are considered retryable,
// i.e. temporary, timeout, DNS, connection refused and reset, host down and unreachable and
// network down and unreachable errors. In addition, any database error is considered retryable.
func Retryable(err error) bool {
	var temporary interface {
		Temporary() bool
	}
	if errors.As(err, &temporary) && temporary.Temporary() {
		return true
	}

	var timeout interface {
		Timeout() bool
	}
	if errors.As(err, &timeout) && timeout.Timeout() {
		return true
	}

	var dnsError *net.DNSError
	if errors.As(err, &dnsError) {
		return true
	}

	var opError *net.OpError
	if errors.As(err, &opError) {
		// OpError provides Temporary() and Timeout(), but not Unwrap(),
		// so we have to extract the underlying error ourselves to also check for ECONNREFUSED,
		// which is not considered temporary or timed out by Go.
		err = opError.Err
	}
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ENOENT) {
		// syscall errors provide Temporary() and Timeout(),
		// which do not include ECONNREFUSED or ENOENT, so we check these ourselves.
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) {
		// ECONNRESET is treated as a temporary error by Go only if it comes from calling accept.
		return true
	}
	if errors.Is(err, syscall.EHOSTDOWN) || errors.Is(err, syscall.EHOSTUNREACH) {
		return true
	}
	if errors.Is(err, syscall.ENETDOWN) || errors.Is(err, syscall.ENETUNREACH) {
		return true
	}
	if errors.Is(err, syscall.EPIPE) {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	if errors.Is(err, driver.ErrBadConn) {
		return true
	}
	if errors.Is(err, mysql.ErrInvalidConn) {
		return true
	}

	var mye *mysql.MySQLError
	var pqe *pq.Error
	if errors.As(err, &mye) || errors.As(err, &pqe) {
		return true
	}

	return false
}
