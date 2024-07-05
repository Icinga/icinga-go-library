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

func TestBinary_MarshalText(t *testing.T) {
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
			actual, err := st.input.MarshalText()

			require.NoError(t, err)
			require.True(t, utf8.Valid(actual))
			require.Equal(t, st.output, string(actual))
		})
	}
}

func TestBinary_UnmarshalText(t *testing.T) {
	subtests := []struct {
		name   string
		input  string
		output Binary
		error  bool
	}{
		{"empty", "", Binary{}, false},
		{"invalid_length", "0", Binary{}, true},
		{"invalid_char", "0g", Binary{}, true},
		{"hex", "0a", Binary{10}, false},
		{"multiple", "01fe", Binary{1, 254}, false},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var actual Binary
			if err := actual.UnmarshalText([]byte(st.input)); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.output, actual)
			}
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

func TestBinary_UnmarshalJSON(t *testing.T) {
	subtests := []struct {
		name   string
		input  string
		output Binary
		error  bool
	}{
		{"null", `null`, nil, false},
		{"bool", `false`, nil, true},
		{"number", `10`, nil, true},
		{"invalid_length", `"0"`, nil, true},
		{"invalid_char", `"0g"`, nil, true},
		{"empty", `""`, make(Binary, 0, 1), false},
		{"nul", `"00"`, Binary{0}, false},
		{"hex", `"0a"`, Binary{10}, false},
		{"multiple", `"01fe"`, Binary{1, 254}, false},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var actual Binary
			if err := actual.UnmarshalJSON([]byte(st.input)); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.output, actual)
			}
		})
	}
}

func TestBinary_Scan(t *testing.T) {
	subtests := []struct {
		name   string
		input  any
		output Binary
		error  bool
	}{
		{"nil", nil, nil, false},
		{"bool", false, nil, true},
		{"number", 10, nil, true},
		{"string", "10", nil, true},
		{"empty", make([]byte, 0, 1), nil, false},
		{"nul", []byte{0}, Binary{0}, false},
		{"hex", []byte{10}, Binary{10}, false},
		{"multiple", []byte{1, 254}, Binary{1, 254}, false},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var actual Binary
			if err := actual.Scan(st.input); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.output, actual)
			}
		})
	}
}
