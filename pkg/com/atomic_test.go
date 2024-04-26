package com

import (
	"github.com/stretchr/testify/require"
	"testing"
)

type atomicTestCase[T any] struct {
	Atomic Atomic[T]
	Values []T
	Zero   T
}

func (test atomicTestCase[T]) popValue() T {
	var v T
	v, test.Values = test.Values[0], test.Values[1:]

	return v
}

func (test atomicTestCase[T]) run(t *testing.T) {
	zero, ok := test.Atomic.Load()
	require.False(t, ok)
	require.Equal(t, test.Zero, zero)

	initial := test.popValue()
	test.Atomic.Store(initial)
	v, ok := test.Atomic.Load()
	require.True(t, ok)
	require.Equal(t, initial, v)

	swap := test.popValue()
	old, ok := test.Atomic.Swap(swap)
	require.True(t, ok)
	require.Equal(t, initial, old)
	v, ok = test.Atomic.Load()
	require.True(t, ok)
	require.Equal(t, swap, v)

	_new := test.popValue()
	swapped := test.Atomic.CompareAndSwap(old, _new)
	require.True(t, swapped)
	v, ok = test.Atomic.Load()
	require.True(t, ok)
	require.Equal(t, _new, v)
}

func TestAtomic(t *testing.T) {
	t.Run("Atomic int", atomicTestCase[int]{
		Values: []int{1, 2, 3},
	}.run)

	t.Run("Atomic string", atomicTestCase[string]{
		Values: []string{"a", "b", "c"},
	}.run)

	t.Run("Atomic bool", atomicTestCase[bool]{
		Values: []bool{true, false, true},
	}.run)
}
