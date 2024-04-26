package com

import (
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

func TestCounter(t *testing.T) {
	var c Counter

	require.Zero(t, c.Val())

	c.Add(5)
	require.EqualValues(t, 5, c.Val())

	c.Inc()
	require.EqualValues(t, 6, c.Val())

	require.EqualValues(t, 6, c.Reset())
	require.Zero(t, c.Val())

	c.Add(4)
	require.EqualValues(t, 10, c.Total())
}

func TestCounter_Concurrency(t *testing.T) {
	var c Counter
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			c.Inc()
			wg.Done()
		}()
	}
	wg.Wait()
	require.EqualValues(t, 10, c.Val())

	wg.Add(1)
	go func() {
		c.Reset()
		wg.Done()
	}()
	wg.Wait()
	require.Zero(t, c.Val())

	require.EqualValues(t, 10, c.Total())
}
