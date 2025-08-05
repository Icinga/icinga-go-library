package types

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
	"unicode/utf8"
)

func TestMakeBool(t *testing.T) {
	t.Parallel()

	subtests := []struct {
		name         string
		input        bool
		transformers []func(*Bool)
		output       Bool
	}{
		{
			name:   "false",
			input:  false,
			output: Bool{Bool: false, Valid: true},
		},
		{
			name:   "true",
			input:  true,
			output: Bool{Bool: true, Valid: true},
		},
		{
			name:         "false-transform-zero-to-null",
			input:        false,
			transformers: []func(*Bool){TransformZeroBoolToNull},
			output:       Bool{Valid: false},
		},
		{
			name:         "true-transform-zero-to-null",
			input:        true,
			transformers: []func(*Bool){TransformZeroBoolToNull},
			output:       Bool{Bool: true, Valid: true},
		},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			require.Equal(t, st.output, MakeBool(st.input, st.transformers...))
		})
	}
}

func TestBool_MarshalJSON(t *testing.T) {
	subtests := []struct {
		input  Bool
		output string
	}{
		{Bool{Bool: false, Valid: false}, `null`},
		{Bool{Bool: false, Valid: true}, `false`},
		{Bool{Bool: true, Valid: false}, `null`},
		{Bool{Bool: true, Valid: true}, `true`},
	}

	for _, st := range subtests {
		t.Run(fmt.Sprintf("Bool-%#v_Valid-%#v", st.input.Bool, st.input.Valid), func(t *testing.T) {
			actual, err := st.input.MarshalJSON()

			require.NoError(t, err)
			require.True(t, utf8.Valid(actual))
			require.Equal(t, st.output, string(actual))
		})
	}
}

func TestBool_UnmarshalText(t *testing.T) {
	subtests := []struct {
		name   string
		input  string
		output Bool
		error  bool
	}{
		{"empty", "", Bool{}, true},
		{"negative", "-1", Bool{}, true},
		{"bool", "false", Bool{}, true},
		{"b", "f", Bool{}, true},
		{"float", "0.0", Bool{}, true},
		{"zero", "0", Bool{Bool: false, Valid: true}, false},
		{"one", "1", Bool{Bool: true, Valid: true}, false},
		{"two", "2", Bool{Bool: true, Valid: true}, false},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var actual Bool
			if err := actual.UnmarshalText([]byte(st.input)); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.output, actual)
			}
		})
	}
}

func TestBool_UnmarshalJSON(t *testing.T) {
	subtests := []struct {
		name   string
		input  string
		output Bool
		error  bool
	}{
		{"null", `null`, Bool{}, false},
		{"false", `false`, Bool{Bool: false, Valid: true}, false},
		{"true", `true`, Bool{Bool: true, Valid: true}, false},
		{"number", `0`, Bool{}, true},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var actual Bool
			if err := actual.UnmarshalJSON([]byte(st.input)); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.output, actual)
			}
		})
	}
}

func TestBool_Scan(t *testing.T) {
	subtests := []struct {
		name   string
		input  any
		output Bool
		error  bool
	}{
		{"nil", nil, Bool{}, false},
		{"bool", false, Bool{}, true},
		{"int64", int64(0), Bool{}, true},
		{"string", "false", Bool{}, true},
		{"n", []byte("n"), Bool{Bool: false, Valid: true}, false},
		{"y", []byte("y"), Bool{Bool: true, Valid: true}, false},
		{"invalid", []byte("false"), Bool{}, true},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var actual Bool
			if err := actual.Scan(st.input); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.output, actual)
			}
		})
	}
}

func TestBool_Value(t *testing.T) {
	subtests := []struct {
		name   string
		input  Bool
		output any
	}{
		{"nil", Bool{}, nil},
		{"invalid", Bool{Bool: true, Valid: false}, nil},
		{"false", Bool{Bool: false, Valid: true}, "n"},
		{"true", Bool{Bool: true, Valid: true}, "y"},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			actual, err := st.input.Value()

			require.NoError(t, err)
			require.Equal(t, st.output, actual)
		})
	}
}
