package com

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCounter_Add(t *testing.T) {
	var c Counter

	c.Add(42)
	require.Equal(t, uint64(42), c.Val(), "unexpected value")
	require.Equal(t, uint64(42), c.Total(), "unexpected total")

	c.Add(23)
	require.Equal(t, uint64(65), c.Val(), "unexpected new value")
	require.Equal(t, uint64(65), c.Total(), "unexpected new total")
}
