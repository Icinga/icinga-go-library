package com

import (
	"context"
	"github.com/stretchr/testify/require"
	"io"
	"testing"
	"time"
)

func TestWaitAsync(t *testing.T) {
	subtests := []struct {
		name  string
		input WaiterFunc
		error error
	}{
		{"no_error", func() error { return nil }, nil},
		{"error", func() error { return io.EOF }, io.EOF},
		{"sleep_no_error", func() error { time.Sleep(time.Second / 2); return nil }, nil},
		{"sleep_error", func() error { time.Sleep(time.Second / 2); return io.EOF }, io.EOF},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			errs := WaitAsync(st.input)
			require.NotNil(t, errs)

			if st.error != nil {
				select {
				case e, ok := <-errs:
					if !ok {
						require.Fail(t, "channel should not be closed, yet")
					}

					require.Equal(t, st.error, e)
				case <-time.After(time.Second):
					require.Fail(t, "channel should not block")
				}
			}

			select {
			case _, ok := <-errs:
				if ok {
					require.Fail(t, "channel should be closed")
				}
			case <-time.After(time.Second):
				require.Fail(t, "channel should not block")
			}
		})
	}
}

func TestCopyFirst(t *testing.T) {
	subtests := []struct {
		name  string
		io    []string
		error bool
	}{
		{"empty", nil, true},
		{"one", []string{"a"}, false},
		{"two", []string{"a", "b"}, false},
		{"three", []string{"a", "b", "c"}, false},
	}

	latencies := []struct {
		name    string
		latency time.Duration
	}{
		{"instantly", 0},
		{"1us", time.Microsecond},
		{"20ms", 20 * time.Millisecond},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			for _, l := range latencies {
				t.Run(l.name, func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					ch := make(chan string)
					go func() {
						defer close(ch)

						for _, v := range st.io {
							if l.latency > 0 {
								select {
								case <-time.After(l.latency):
								case <-ctx.Done():
									return
								}
							}

							select {
							case ch <- v:
							case <-ctx.Done():
								return
							}
						}
					}()

					first, forward, err := CopyFirst(ctx, ch)
					if st.error {
						require.Error(t, err)
						require.Nil(t, forward, "forward should be nil")
						return
					}

					require.NoError(t, err)
					require.NotNil(t, forward, "forward should not be nil")

					expected := ""
					if len(st.io) > 0 {
						expected = st.io[0]
					}

					require.Equal(t, expected, first, "first should be the first element")

					for _, expected := range st.io {
						select {
						case actual, ok := <-forward:
							if !ok {
								require.Fail(t, "channel should not be closed")
							}

							require.Equal(t, expected, actual, "forwarded element should match")
						case <-time.After(time.Second):
							require.Fail(t, "channel should not block")
						}
					}

					select {
					case _, ok := <-forward:
						if ok {
							require.Fail(t, "channel should be closed")
						}
					case <-time.After(time.Second):
						require.Fail(t, "channel should not block")
					}
				})
			}
		})
	}

	t.Run("cancel-ctx", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		first, forward, err := CopyFirst(ctx, make(chan int))

		require.Error(t, err)
		require.Nil(t, forward)
		require.Empty(t, first)
	})
}
