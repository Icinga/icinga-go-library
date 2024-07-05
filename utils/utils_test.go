package utils

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestBatchSliceOfStrings(t *testing.T) {
	subtests := []struct {
		name   string
		keys   []string
		count  int
		output [][]string
	}{
		{"nil", nil, 1, nil},
		{"empty", make([]string, 0, 1), 1, nil},
		{"a", []string{"a"}, 1, [][]string{{"a"}}},
		{"a2", []string{"a"}, 2, [][]string{{"a"}}},
		{"a_b", []string{"a", "b"}, 1, [][]string{{"a"}, {"b"}}},
		{"ab", []string{"a", "b"}, 2, [][]string{{"a", "b"}}},
		{"ab3", []string{"a", "b"}, 3, [][]string{{"a", "b"}}},
		{"a_b_c", []string{"a", "b", "c"}, 1, [][]string{{"a"}, {"b"}, {"c"}}},
		{"ab_c", []string{"a", "b", "c"}, 2, [][]string{{"a", "b"}, {"c"}}},
		{"abc", []string{"a", "b", "c"}, 3, [][]string{{"a", "b", "c"}}},
		{"abc4", []string{"a", "b", "c"}, 4, [][]string{{"a", "b", "c"}}},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			batches := BatchSliceOfStrings(context.Background(), st.keys, st.count)
			require.NotNil(t, batches)

			for _, expected := range st.output {
				select {
				case actual, ok := <-batches:
					require.True(t, ok, "receiving should return a value")
					require.Equal(t, expected, actual)
				case <-time.After(10 * time.Millisecond):
					require.Fail(t, "receiving should not block")
				}
			}

			select {
			case _, ok := <-batches:
				require.False(t, ok, "receiving from channel should not return anything")
			case <-time.After(10 * time.Millisecond):
				require.Fail(t, "receiving should not block")
			}
		})
	}
}

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
