package utils

import (
	"context"
	"crypto/sha1"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestBatchSliceOfStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		count    int
		expected [][]string
	}{
		{
			name:     "Even batches",
			input:    []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"},
			count:    3,
			expected: [][]string{{"a", "b", "c"}, {"d", "e", "f"}, {"g", "h", "i"}},
		},
		{
			name:     "Uneven batches",
			input:    []string{"a", "b", "c", "d", "e", "f", "g"},
			count:    3,
			expected: [][]string{{"a", "b", "c"}, {"d", "e", "f"}, {"g"}},
		},
		{
			name:     "Empty keys",
			input:    []string{},
			count:    3,
			expected: [][]string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Literal declaration on purpose to also initialize the variable,
			// so that the empty test does not fail accidentally.
			result := [][]string{}
			for batch := range BatchSliceOfStrings(context.Background(), test.input, test.count) {
				result = append(result, batch)
			}
			require.Equal(t, test.expected, result)
		})
	}

	t.Run("Cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		ch := BatchSliceOfStrings(
			ctx,
			[]string{"a", "b", "c", "d", "e", "f", "g", "h", "i"},
			3,
		)

		select {
		case _, more := <-ch:
			require.False(t, more, "Expected channel to be closed")
		case <-time.After(100 * time.Millisecond):
			t.Error("Timeout waiting for closed channel")
		}
	})

	t.Run("Cancelling context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch := BatchSliceOfStrings(
			ctx,
			[]string{"a", "b", "c", "d", "e", "f", "g", "h", "i"},
			3,
		)
		expected := [][]string{{"a", "b", "c"}}
		var result [][]string
		for batch := range ch {
			result = append(result, batch)
			cancel()
		}
		require.Equal(t, expected, result, "Canceled context did not close the channel as expected")
	})
}

func TestChecksum(t *testing.T) {
	t.Run("String input", func(t *testing.T) {
		input := "hello"
		expected := sha1.Sum([]byte(input))
		result := Checksum(input)
		require.Equal(t, expected[:], result)
	})

	t.Run("Byte input", func(t *testing.T) {
		input := []byte{104, 101, 108, 108, 111}
		expected := sha1.Sum(input)
		result := Checksum(input)
		require.Equal(t, expected[:], result)
	})

	t.Run("Invalid input", func(t *testing.T) {
		input := 123

		defer func() {
			if result := recover(); result == nil {
				t.Errorf("Did not panic with invalid input")
			}
		}()

		_ = Checksum(input)
	})
}

func TestEllipsize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		limit    int
		expected string
	}{
		{
			name:     "String shorter than limit",
			input:    "Hello world",
			limit:    20,
			expected: "Hello world",
		},
		{
			name:     "String equal to limit",
			input:    "Hello world",
			limit:    11,
			expected: "Hello world",
		},
		{
			name:     "String longer than limit",
			input:    "This is a long string that needs to be shortened",
			limit:    20,
			expected: "This is a long st...",
		},
		{
			name:     "String exactly three characters, i.e. same as ellipsis length",
			input:    "abc",
			limit:    3,
			expected: "abc",
		},
		{
			name:     "Limit is smaller than ellipsis length",
			input:    "This is a long string",
			limit:    2,
			expected: "...",
		},
		{
			name:     "UTF-8 string with emojis",
			input:    "🙂🙃😀😃😄😁😆😅",
			limit:    5,
			expected: "🙂🙃...",
		},
		{
			name:     "UTF-8 string with combining characters",
			input:    "café", // 5 Unicode code points
			limit:    4,
			expected: "c...",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := Ellipsize(test.input, test.limit)
			require.Equal(t, test.expected, result)
		})
	}
}

func TestIsUnixAddr(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		{
			name:     "Unix socket address",
			host:     "/var/run/socket",
			expected: true,
		},
		{
			name:     "Non-Unix socket address",
			host:     "localhost:8080",
			expected: false,
		},
		{
			name:     "Empty string",
			host:     "",
			expected: false,
		},
		{
			name:     "Relative path",
			host:     "./socket",
			expected: false,
		},
		{
			name:     "Windows path",
			host:     "C:\\Program Files\\socket",
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := IsUnixAddr(test.host)
			require.Equal(t, test.expected, result)
		})
	}
}

func TestJoinHostPort(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     int
		expected string
	}{
		{
			name:     "Hostname and port",
			host:     "localhost",
			port:     8080,
			expected: "localhost:8080",
		},
		{
			name:     "IPv4 and port",
			host:     "127.0.0.1",
			port:     8080,
			expected: "127.0.0.1:8080",
		},
		{
			name:     "IPv6 and port",
			host:     "::1",
			port:     8080,
			expected: "[::1]:8080",
		},
		{
			name:     "Unix socket address",
			host:     "/var/run/socket",
			expected: "/var/run/socket",
		},
		{
			name:     "Empty host with port",
			host:     "",
			port:     8080,
			expected: ":8080",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := JoinHostPort(test.host, test.port)
			require.Equal(t, test.expected, result)
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
