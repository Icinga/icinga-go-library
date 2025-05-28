package backoff

import (
	"math/rand"
	"time"
)

// Backoff returns the backoff duration for a specific retry attempt.
type Backoff func(uint64) time.Duration

// DefaultBackoff is our opinionated Backoff function for retry.WithBackoff - between 128ms and 1m.
var DefaultBackoff = NewExponentialWithJitter(128*time.Millisecond, 1*time.Minute)

// NewExponentialWithJitter returns an exponentially increasing [Backoff] implementation.
//
// The calculated [time.Duration] values are within [min, max], exponentially increasing and slightly randomized.
// If min or max are zero or negative, they will default to 100ms and 10s, respectively. It panics if min >= max.
func NewExponentialWithJitter(min, max time.Duration) Backoff {
	if min <= 0 {
		min = 100 * time.Millisecond
	}
	if max <= 0 {
		max = 10 * time.Second
	}
	if min >= max {
		panic("max must be greater than min")
	}

	return func(attempt uint64) time.Duration {
		e := min << attempt
		// If the bit shift already overflows, return max.
		if e < min {
			return max
		}

		// Introduce jitter. e <- [min/2, int64_max)
		e = e/2 + time.Duration(rand.Int63n(int64(e/2))) // #nosec G404 -- we don't need crypto/rand here though.
		// Remap e to [min, max].
		if e < min {
			e = min
		}
		if e > max {
			e = max
		}

		return e
	}
}
