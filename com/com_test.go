package com

import (
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
