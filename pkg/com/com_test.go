package com

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"testing"
	"time"
)

func TestWaitAsync(t *testing.T) {
	t.Run("Propagates error and closes returned channel", func(t *testing.T) {
		expected := errors.New("wait error")

		w := &mockWaiter{err: expected}
		errs := WaitAsync(w)

		select {
		case err := <-errs:
			if !errors.Is(err, expected) {
				t.Errorf("Expected error %v, got %v", expected, err)
			}
			select {
			case _, more := <-errs:
				require.False(t, more, "Expected channel to be closed after propagating error")
			case <-time.After(100 * time.Millisecond):
				t.Error("Timeout waiting for closed channel")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Timeout waiting for error")
		}
	})

	t.Run("Nil error closes returned channel", func(t *testing.T) {
		w := &mockWaiter{err: nil}
		errs := WaitAsync(w)

		select {
		case _, more := <-errs:
			require.False(t, more, "Expected channel to be closed if error is nil")
		case <-time.After(100 * time.Millisecond):
			t.Error("Timeout waiting for closed channel")
		}
	})
}

func TestErrgroupReceive(t *testing.T) {
	t.Run("Propagates first error", func(t *testing.T) {
		g, _ := errgroup.WithContext(context.Background())

		g.Go(func() error {
			return nil
		})

		errs := make(chan error, 2)
		expected := errors.New("error")
		errs <- expected
		errs <- errors.New("error #2")
		close(errs)

		ErrgroupReceive(g, errs)

		if err := g.Wait(); !errors.Is(err, expected) {
			t.Errorf("Expected error to be %v, got %v", expected, err)
		}
	})

	t.Run("Closed channel", func(t *testing.T) {
		g, _ := errgroup.WithContext(context.Background())

		errs := make(chan error)
		close(errs)

		ErrgroupReceive(g, errs)

		require.NoError(t, g.Wait())
	})
}

func TestCopyFirst(t *testing.T) {
	tests := []struct {
		name         string
		input        []int
		shouldCancel func(item int) bool
		expected     []int
	}{
		{
			name:     "First + forward",
			input:    []int{0, 1, 2},
			expected: []int{0, 1, 2},
		},
		{
			name:  "Context cancellation",
			input: []int{0, 1, 2},
			shouldCancel: func(item int) bool {
				return item == 2
			},
			expected: []int{0, 1},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancelCtx := context.WithCancel(context.Background())
			defer cancelCtx()

			input := make(chan int)
			go func() {
				defer close(input)
				for _, i := range test.input {
					if test.shouldCancel != nil && test.shouldCancel(i) {
						cancelCtx()
					}

					input <- i
				}
			}()

			first, forward, err := CopyFirst(ctx, input)
			if err != nil {
				t.Fatal(err)
			}

			require.Equal(t, test.expected[0], first)

			var result []int
			for i := range forward {
				result = append(result, i)
			}
			require.Equal(t, test.expected, result)

			select {
			case _, more := <-forward:
				require.False(t, more, "Expected forward channel to be closed")
			case <-time.After(100 * time.Millisecond):
				t.Error("Timeout waiting for closed channel")
			}
		})
	}

	t.Run("Canceled context", func(t *testing.T) {
		ctx, cancelCtx := context.WithCancel(context.Background())
		cancelCtx()

		input := make(chan int)
		go func() {
			defer close(input)
			for i := range 3 {
				input <- i
			}
		}()

		first, forward, err := CopyFirst(ctx, input)
		var zero int
		require.Equal(t, zero, first)
		require.Nil(t, forward)
		require.Equal(t, context.Canceled, err)
	})

	t.Run("Closed channel", func(t *testing.T) {
		input := make(chan int)
		close(input)

		_, _, err := CopyFirst[int](context.Background(), input)
		require.Error(t, err)
	})
}

type mockWaiter struct {
	err     error
	timeout time.Duration
}

func (m *mockWaiter) Wait() error {
	if m.timeout > 0 {
		time.Sleep(m.timeout)
	}

	return m.err
}
