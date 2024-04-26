package periodic

import (
	"context"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"
)

func TestPeriodic(t *testing.T) {
	interval := 100 * time.Millisecond

	t.Run("Start after interval + elapsed", func(t *testing.T) {
		var firstTick *Tick
		wait := make(chan struct{})

		startTime := time.Now()
		defer Start(context.Background(), interval, func(tick Tick) {
			if firstTick == nil {
				firstTick = &tick
				close(wait)
			}
		}).Stop()

		select {
		case <-wait:
		case <-time.After(2 * interval):
			t.Error("Timeout waiting for tick")
		}

		if firstTick.Time.Sub(startTime) < interval {
			t.Error("Expected first tick after interval, but was earlier")
		}

		require.GreaterOrEqual(t, firstTick.Elapsed, interval)
	})

	t.Run("Start immediate + elapsed", func(t *testing.T) {
		var firstTick *Tick
		wait := make(chan struct{})

		startTime := time.Now()
		task := Start(context.Background(), interval, func(tick Tick) {
			firstTick = &tick
			close(wait)
		}, Immediate())
		defer task.Stop()

		select {
		case <-wait:
			task.Stop()
		case <-time.After(interval):
			t.Error("Timeout waiting for tick")
		}

		if firstTick.Time.Sub(startTime) > interval {
			t.Error("Expected first tick before interval, but was after")
		}

		require.Less(t, firstTick.Elapsed, interval)
	})

	t.Run("Stop", func(t *testing.T) {
		var ticks atomic.Int64
		wait := make(chan struct{})

		stop := Start(context.Background(), interval, func(Tick) {
			ticks.Add(1)

			if ticks.Load() == 2 {
				close(wait)
			}
		})

		select {
		case <-wait:
		case <-time.After(3 * interval):
			t.Error("Timeout waiting for ticks")
		}
		stop.Stop()
		expected := ticks.Load()
		require.NotZero(t, expected)

		time.Sleep(2 * interval)
		require.Equal(t, ticks.Load(), expected, "Expected no more ticks after stop")
	})

	t.Run("OnStop", func(t *testing.T) {
		var onStopTick *Tick
		wait := make(chan struct{})

		Start(context.Background(), interval, func(Tick) {
		}, OnStop(func(tick Tick) {
			onStopTick = &tick
			close(wait)
		})).Stop()

		select {
		case <-wait:
		case <-time.After(interval):
			t.Error("Timeout waiting for stop")
		}

		require.NotZero(t, onStopTick.Elapsed)
		require.Less(t, onStopTick.Elapsed, interval)
	})
}
