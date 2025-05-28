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
			reachedMax := false

			for i := uint64(0); i < 1024; i++ {
				d := r(i)
				require.GreaterOrEqual(t, d, tt.min)
				require.LessOrEqual(t, d, tt.max)

				if reachedMax && d != tt.max {
					t.Errorf("max value %v was already reached, but r(%d) := %v", tt.max, i, d)
				}

				if d == tt.max {
					reachedMax = true
				}
			}
			if !reachedMax {
				t.Error("max value was never reached")
			}
		})
	}
}
