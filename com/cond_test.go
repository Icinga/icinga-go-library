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

func TestCond_Close(t *testing.T) {
	cond := NewCond(context.Background())
	done := cond.Done()
	wait := cond.Wait()

	require.NoError(t, cond.Close())

	select {
	case _, ok := <-done:
		if ok {
			require.Fail(t, "existing cond-closed channel should be closed")
		}
	case <-time.After(time.Second / 10):
		require.Fail(t, "cond should be closed")
	}

	select {
	case _, ok := <-cond.Done():
		if ok {
			require.Fail(t, "new cond-closed channel should be closed")
		}
	case <-time.After(time.Second / 10):
		require.Fail(t, "cond should be still closed")
	}

	select {
	case <-wait:
		require.Fail(t, "cond should not be ready")
	case <-time.After(time.Second / 10):
	}

	require.Panics(t, func() { cond.Wait() }, "cond should panic on Wait after Close")
	require.Panics(t, func() { cond.Broadcast() }, "cond should panic on Broadcast after Close")
}
