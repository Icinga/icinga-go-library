package utils

import (
	"context"
	"encoding/hex"
	"fmt"
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

	for _, i := range []int{0, -1, -2, -30} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			require.Panics(t, func() { BatchSliceOfStrings(context.Background(), nil, i) })
		})
	}
}

func TestChecksum(t *testing.T) {
	subtests := []struct {
		name   string
		input  any
		output string
	}{
		{"empty_string", "", "da39a3ee5e6b4b0d3255bfef95601890afd80709"},
		{"empty_bytes", []byte(nil), "da39a3ee5e6b4b0d3255bfef95601890afd80709"},
		{"space_string", " ", "b858cb282617fb0956d960215c8e84d1ccf909c6"},
		{"space_bytes", []byte(" "), "b858cb282617fb0956d960215c8e84d1ccf909c6"},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			require.Equal(t, st.output, hex.EncodeToString(Checksum(st.input)))
		})
	}

	unsupported := []struct {
		name  string
		input any
	}{
		{"nil", nil},
		{"bool", false},
		{"int", 0},
		{"float", 0.0},
		{"struct", struct{}{}},
		{"slice", []string{}},
		{"map", map[string]string{}},
	}

	for _, st := range unsupported {
		t.Run(st.name, func(t *testing.T) {
			require.Panics(t, func() { Checksum(st.input) })
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
