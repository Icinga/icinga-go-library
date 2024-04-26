package backoff

import (
	"github.com/stretchr/testify/require"
	"math"
	"testing"
	"time"
)

func TestNewExponentialWithJitter(t *testing.T) {
	_min := 100 * time.Millisecond
	_max := 1 * time.Second

	t.Run("Duration increases with each attempt until max is reached", func(t *testing.T) {
		backoff := NewExponentialWithJitter(_min, _max)

		last := backoff(0)
		require.Less(t, last, _min, "Duration of first attempt must be less than min")

		increaseUntil := uint64(math.Log2(float64(_max) / float64(_min)))
		for i := uint64(1); i < 10; i++ {
			duration := backoff(i)
			require.Greater(t, duration, _min, "Duration from the second attempt must be more than min")
			require.Less(t, duration, _max, "Duration must not exceed max")

			if i <= increaseUntil {
				require.Greater(t, duration, last, "Duration must increase with each attempt until max is reached")
				last = duration
			}
		}
	})

	t.Run("Panics if min is greater than max", func(t *testing.T) {
		defer func() {
			if result := recover(); result == nil {
				t.Error("Did not panic with min >= max")
			}
		}()

		_ = NewExponentialWithJitter(_max, _min)
	})
}
