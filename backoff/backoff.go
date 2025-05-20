package backoff

import (
	"math/rand"
	"time"
)

// Backoff returns the backoff duration for a specific retry attempt.
type Backoff func(uint64) time.Duration

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
		e := time.Duration(jitter(int64(min << attempt)))
		if e < min {
			e = min
		}
		if e > max {
			e = max
		}

		return e
	}
}

// jitter returns a random integer distributed in the range [n/2..n).
func jitter(n int64) int64 {
	if n == 0 {
		return 0
	}

	return n/2 + rand.Int63n(n/2) // #nosec G404 -- Use of weak random number generator - we don't need crypto/rand here though.
}
