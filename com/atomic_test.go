package com

import (
	"github.com/stretchr/testify/require"
	"testing"
)

type testInterface interface {
	DoNothing()
}

type testImpl struct {
	i int
}

func (*testImpl) DoNothing() {}

func TestAtomic_Load(t *testing.T) {
	subtests := []struct {
		name string
		init bool
		io   testInterface
	}{
		{"uninitialized", false, nil},
		{"nil", true, nil},
		{"nilptr", true, (*testImpl)(nil)},
		{"zero", true, &testImpl{}},
		{"nonzero", true, &testImpl{42}},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var a Atomic[testInterface]
			if st.init {
				a.Store(st.io)
			}

			v, ok := a.Load()
			require.Equal(t, st.init, ok)
			require.Equal(t, st.io, v)
		})
	}
}
