package com

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestBulker(t *testing.T) {
	tests := []struct {
		name               string
		count              int
		splitPolicyFactory BulkChunkSplitPolicyFactory[int]
		shouldCancel       func(item int) bool
		expected           [][]int
	}{
		{
			name:               "Even batches",
			count:              5,
			splitPolicyFactory: NeverSplit[int],
			expected:           [][]int{{0, 1, 2, 3, 4}, {5, 6, 7, 8, 9}},
		},
		{
			name:               "Uneven batches",
			count:              3,
			splitPolicyFactory: NeverSplit[int],
			expected:           [][]int{{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, {9}},
		},
		{
			name:  "Custom split policy",
			count: 3,
			splitPolicyFactory: func() BulkChunkSplitPolicy[int] {
				count := 0
				return func(item int) bool {
					split := count > 0 && count&1 == 0
					count++

					return split
				}
			},
			expected: [][]int{{0, 1}, {2, 3}, {4, 5}, {6, 7}, {8, 9}},
		},
		{
			name:               "Timeout",
			count:              3,
			splitPolicyFactory: NeverSplit[int],
			shouldCancel: func(item int) bool {
				if item < 3 {
					time.Sleep(256 * time.Millisecond)
					time.Sleep(100 * time.Millisecond)
				}

				return false
			},
			expected: [][]int{{0}, {1}, {2, 3, 4}, {5, 6, 7}, {8, 9}},
		},
		{
			name:               "Context cancellation",
			count:              3,
			splitPolicyFactory: NeverSplit[int],
			shouldCancel: func(item int) bool {
				if item == 5 {
					// Cancel context in the middle of chunk.
					return true
				}

				if item == 6 {
					// Wait
					time.Sleep(100 * time.Millisecond)
				}

				return false
			},
			expected: [][]int{{0, 1, 2}},
		},
		{
			name:               "Canceled context",
			count:              3,
			splitPolicyFactory: NeverSplit[int],
			shouldCancel: func(item int) bool {
				return item == 0
			},
			expected: [][]int{},
		},
		{
			name:               "Timeout and context cancellation",
			count:              3,
			splitPolicyFactory: NeverSplit[int],
			shouldCancel: func(item int) bool {
				if item == 5 {
					time.Sleep(256 * time.Millisecond)
					time.Sleep(100 * time.Millisecond)
					// Cancel context in the middle of chunk.
					return true
				}

				if item == 6 {
					// Wait
					time.Sleep(100 * time.Millisecond)
				}

				return false
			},
			expected: [][]int{{0, 1, 2}, {3, 4}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancelCtx := context.WithCancel(context.Background())
			defer cancelCtx()

			input := make(chan int)
			go func() {
				defer close(input)
				for i := 0; i < 10; i++ {
					if test.shouldCancel != nil && test.shouldCancel(i) {
						cancelCtx()
					}
					input <- i
				}
			}()

			bulker := NewBulker[int](ctx, input, test.count, test.splitPolicyFactory)
			chunks := bulker.Bulk()

			// Literal declaration on purpose to also initialize the variable,
			// so that the empty test does not fail accidentally.
			result := [][]int{}
			for chunk := range chunks {
				result = append(result, chunk)
			}
			require.Equal(t, test.expected, result)

			select {
			case _, more := <-chunks:
				require.False(t, more, "Expected channel to be closed if error is nil")
			case <-time.After(100 * time.Millisecond):
				t.Error("Timeout waiting for closed channel")
			}
		})
	}
}
