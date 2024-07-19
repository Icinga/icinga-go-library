package com

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestCond_Broadcast(t *testing.T) {
	cond := NewCond(context.Background())
	defer func() { _ = cond.Close() }()

	done := cond.Done()
	wait := cond.Wait()

	select {
	case <-done:
		require.Fail(t, "cond should not be closed, yet")
	case <-wait:
		require.Fail(t, "cond should not be ready, yet")
	case <-time.After(time.Second / 10):
	}

	cond.Broadcast()

	select {
	case <-done:
		require.Fail(t, "cond should still not be closed")
	case <-cond.Done():
		require.Fail(t, "cond should not be closed for round 2, yet")
	case <-cond.Wait():
		require.Fail(t, "cond should not be ready for round 2")
	case <-time.After(time.Second / 10):
	}

	select {
	case _, ok := <-wait:
		if ok {
			require.Fail(t, "cond ready channel should be closed")
		}
	case <-time.After(time.Second / 10):
		require.Fail(t, "cond should be ready")
	}
}
