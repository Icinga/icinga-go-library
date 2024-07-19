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

func TestAtomic_Swap(t *testing.T) {
	subtests := []struct {
		name string
		init bool
		io   testInterface
		new  testInterface
	}{
		{"uninitialized", false, nil, (*testImpl)(nil)},
		{"nil", true, (*testImpl)(nil), nil},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var a Atomic[testInterface]
			if st.init {
				a.Store(st.io)
			}

			old, ok := a.Swap(st.new)
			require.Equal(t, st.init, ok, "Swap second return value")
			require.Equal(t, st.io, old, "Swap first return value")

			v, ok := a.Load()
			require.True(t, ok, "Load second return value")
			require.Equal(t, st.new, v, "Load first return value")
		})
	}
}

func TestAtomic_CompareAndSwap(t *testing.T) {
	subtests := []struct {
		name    string
		init    bool
		io      testInterface
		old     testInterface
		new     testInterface
		swapped bool
	}{
		{"uninitialized_nil_nonzero", false, nil, nil, &testImpl{}, false},
		{"uninitialized_nilptr_nonzero", false, nil, (*testImpl)(nil), &testImpl{}, false},
		{"uninitialized_nonzero_nilptr", false, nil, &testImpl{}, (*testImpl)(nil), false},
		{"nil_nil_nonzero", true, nil, nil, &testImpl{}, true},
		{"nil_nilptr_nonzero", true, nil, (*testImpl)(nil), &testImpl{}, false},
		{"nil_nonzero_nilptr", true, nil, &testImpl{}, (*testImpl)(nil), false},
		{"nilptr_nil_nonzero", true, (*testImpl)(nil), nil, &testImpl{}, false},
		{"nilptr_nilptr_nonzero", true, (*testImpl)(nil), (*testImpl)(nil), &testImpl{}, true},
		{"nilptr_nonzero_nil", true, (*testImpl)(nil), &testImpl{}, nil, false},
		{"nonzero_nil_nilptr", true, &testImpl{}, nil, (*testImpl)(nil), false},
		{"nonzero_nilptr_nil", true, &testImpl{}, (*testImpl)(nil), nil, false},
		{"nonzero_nonzero_nilptr", true, &testImpl{}, &testImpl{}, (*testImpl)(nil), false},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var a Atomic[testInterface]
			if st.init {
				a.Store(st.io)
			}

			require.Equal(t, st.swapped, a.CompareAndSwap(st.old, st.new), "CompareAndSwap return value")

			if v, ok := a.Load(); st.swapped {
				require.True(t, ok, "Load second return value")
				require.Equal(t, st.new, v, "Load first return value")
			} else {
				require.Equal(t, st.init, ok, "Load second return value")
				require.Equal(t, st.io, v, "Load first return value")
			}
		})
	}
}
