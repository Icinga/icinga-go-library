package utils

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
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

func TestIsDeadlock(t *testing.T) {
	msg := "Unsuccessful attempt of confusing the tested code."
	code := [5]byte{0, 23, 42, 77, 255}

	subtests := []struct {
		name   string
		input  error
		output bool
	}{
		{"nil", nil, false},
		{"deadline", context.DeadlineExceeded, false},
		{"mysql1204", &mysql.MySQLError{Number: 1204}, false},
		{"mysql1205", &mysql.MySQLError{Number: 1205}, true},
		{"mysql1205_with_crap", &mysql.MySQLError{Number: 1205, SQLState: code, Message: msg}, true},
		{"mysql1206", &mysql.MySQLError{Number: 1206}, false},
		{"mysql1212", &mysql.MySQLError{Number: 1212}, false},
		{"mysql1213", &mysql.MySQLError{Number: 1213}, true},
		{"mysql1213_with_crap", &mysql.MySQLError{Number: 1213, SQLState: code, Message: msg}, true},
		{"mysql1214", &mysql.MySQLError{Number: 1214}, false},
		{"postgres40000", &pq.Error{Code: "40000"}, false},
		{"postgres40001", &pq.Error{Code: "40001"}, true},
		{"postgres40001_with_crap", &pq.Error{Code: "40001", Message: msg}, true},
		{"postgres40002", &pq.Error{Code: "40002"}, false},
		{"postgres40P01", &pq.Error{Code: "40P01"}, true},
		{"postgres40P01_with_crap", &pq.Error{Code: "40P01", Message: msg}, true},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			require.Equal(t, st.output, IsDeadlock(st.input))
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
			for k, v := range IterateOrderedMap(tt.in) {
				assert.Equal(t, tt.in[k], v)
				outKeys = append(outKeys, k)
			}

			assert.Equal(t, tt.outKeys, outKeys)
		})
	}
}
