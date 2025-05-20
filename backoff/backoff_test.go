package backoff

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestNewExponentialWithJitter(t *testing.T) {
	tests := []struct {
		name string
		min  time.Duration
		max  time.Duration
	}{
		{"defaults", 100 * time.Millisecond, 10 * time.Second},
		{"small-values", time.Millisecond, time.Second},
		{"huge-values", time.Minute, time.Hour},
		{"small-range", time.Millisecond, 2 * time.Millisecond},
		{"huge-range", time.Millisecond, time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewExponentialWithJitter(tt.min, tt.max)

			// Ensure that multiple calls don't breach the upper bound
			maxCounter := 0

			for i := uint64(0); ; i++ {
				if maxCounter >= 10 {
					break
				}
				if i > 1_000_000 {
					t.Error("not reached max")
				}

				d := r(i)
				require.GreaterOrEqual(t, d, tt.min)
				require.LessOrEqual(t, d, tt.max)

				if d == tt.max {
					maxCounter++
				}
			}
		})
	}
}
