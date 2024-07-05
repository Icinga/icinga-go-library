package types

import (
	"github.com/stretchr/testify/require"
	"testing"
	"unicode/utf8"
)

func TestBinary_Valid(t *testing.T) {
	subtests := []struct {
		name   string
		input  Binary
		output bool
	}{
		{"nil", nil, false},
		{"empty", make(Binary, 0, 1), false},
		{"nul", Binary{0}, true},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			require.Equal(t, st.output, st.input.Valid())
		})
	}
}

func TestBinary_String(t *testing.T) {
	subtests := []struct {
		name   string
		input  Binary
		output string
	}{
		{"nil", nil, ""},
		{"nul", Binary{0}, "00"},
		{"hex", Binary{10}, "0a"},
		{"multiple", Binary{1, 254}, "01fe"},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			require.Equal(t, st.output, st.input.String())
		})
	}
}

func TestBinary_MarshalJSON(t *testing.T) {
	subtests := []struct {
		name   string
		input  Binary
		output string
	}{
		{"nil", nil, `null`},
		{"empty", make(Binary, 0, 1), `null`},
		{"space", Binary(" "), `"20"`},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			actual, err := st.input.MarshalJSON()

			require.NoError(t, err)
			require.True(t, utf8.Valid(actual))
			require.Equal(t, st.output, string(actual))
		})
	}
}
