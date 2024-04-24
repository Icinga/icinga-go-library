package utils

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestChanFromSlice(t *testing.T) {
	t.Run("Nil", func(t *testing.T) {
		ch := ChanFromSlice[int](nil)
		require.NotNil(t, ch)
		requireClosedEmpty(t, ch)
	})

	t.Run("Empty", func(t *testing.T) {
		ch := ChanFromSlice([]int{})
		require.NotNil(t, ch)
		requireClosedEmpty(t, ch)
	})

	t.Run("NonEmpty", func(t *testing.T) {
		ch := ChanFromSlice([]int{42, 23, 1337})
		require.NotNil(t, ch)
		requireReceive(t, ch, 42)
		requireReceive(t, ch, 23)
		requireReceive(t, ch, 1337)
		requireClosedEmpty(t, ch)
	})
}

// requireReceive is a helper function to check if a value can immediately be received from a channel.
func requireReceive(t *testing.T, ch <-chan int, expected int) {
	t.Helper()

	select {
	case v, ok := <-ch:
		require.True(t, ok, "receiving should return a value")
		require.Equal(t, expected, v)
	default:
		require.Fail(t, "receiving should not block")
	}
}

// requireReceive is a helper function to check if the channel is closed and empty.
func requireClosedEmpty(t *testing.T, ch <-chan int) {
	t.Helper()

	select {
	case _, ok := <-ch:
		require.False(t, ok, "receiving from channel should not return anything")
	default:
		require.Fail(t, "receiving should not block")
	}
}

func TestIterateOrderedMap(t *testing.T) {
	tests := []struct {
		name    string
		in      map[int]string
		outKeys []int
	}{
		{"empty", map[int]string{}, nil},
		{"single", map[int]string{1: "foo"}, []int{1}},
		{"few-numbers", map[int]string{1: "a", 2: "b", 3: "c"}, []int{1, 2, 3}},
		{
			"1k-numbers",
			func() map[int]string {
				m := make(map[int]string)
				for i := 0; i < 1000; i++ {
					m[i] = "foo"
				}
				return m
			}(),
			func() []int {
				keys := make([]int, 1000)
				for i := 0; i < 1000; i++ {
					keys[i] = i
				}
				return keys
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var outKeys []int

			// Either run with GOEXPERIMENT=rangefunc or wait for rangefuncs to land in the next Go release.
			// for k, _ := range IterateOrderedMap(tt.in) {
			// 	outKeys = append(outKeys, k)
			// }

			// In the meantime, it can be invoked as follows.
			IterateOrderedMap(tt.in)(func(k int, v string) bool {
				assert.Equal(t, tt.in[k], v)
				outKeys = append(outKeys, k)
				return true
			})

			assert.Equal(t, tt.outKeys, outKeys)
		})
	}
}
