package utils

import (
	"context"
	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
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

func TestChecksum(t *testing.T) {
	subtests := []struct {
		name  string
		input any
	}{
		{"empty_string", ""},
		{"empty_bytes", []byte(nil)},
		{"space_string", " "},
		{"space_bytes", []byte(" ")},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			actual := Checksum(st.input)

			require.Len(t, actual, 20)
			require.NotEqual(t, make([]byte, 20), actual)
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

func TestEllipsize(t *testing.T) {
	subtests := []struct {
		name   string
		s      string
		limit  int
		output string
	}{
		{"negative", "", -1, "..."},
		{"empty", "", 0, ""},
		{"shorter", " ", 2, " "},
		{"equal", " ", 1, " "},
		{"longer", " ", 0, "..."},
		{"unicode", "äöüß€", 4, "ä..."},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			require.Equal(t, st.output, Ellipsize(st.s, st.limit))
		})
	}
}

func TestMaxInt(t *testing.T) {
	subtests := []struct {
		name   string
		x      int
		y      int
		output int
	}{
		{"less", 23, 42, 42},
		{"equal", 42, 42, 42},
		{"greater", 42, 23, 42},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			require.Equal(t, st.output, MaxInt(st.x, st.y))
		})
	}
}

func TestIsUnixAddr(t *testing.T) {
	subtests := []struct {
		name   string
		input  string
		output bool
	}{
		{"empty", "", false},
		{"slash", "/", true},
		{"unix", "/tmp/sock", true},
		{"ipv4", "192.0.2.1", false},
		{"ipv6", "2001:db8::", false},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			require.Equal(t, st.output, IsUnixAddr(st.input))
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
