package types

import (
	"database/sql"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFloat_MarshalJSON(t *testing.T) {
	subtests := []struct {
		name   string
		input  sql.NullFloat64
		output string
	}{
		{"null", sql.NullFloat64{}, `null`},
		{"invalid", sql.NullFloat64{Float64: 42}, `null`},
		{"zero", sql.NullFloat64{Float64: 0, Valid: true}, `0`},
		{"negative", sql.NullFloat64{Float64: -1, Valid: true}, `-1`},
		{"fraction", sql.NullFloat64{Float64: 0.5, Valid: true}, `0.5`},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			actual, err := Float{st.input}.MarshalJSON()

			require.NoError(t, err)
			require.Equal(t, st.output, string(actual))
		})
	}
}

func TestFloat_UnmarshalText(t *testing.T) {
	subtests := []struct {
		name   string
		input  string
		output sql.NullFloat64
		error  bool
	}{
		{"empty", "", sql.NullFloat64{}, true},
		{"too_big", "1e309", sql.NullFloat64{}, true},
		{"zero", "0", sql.NullFloat64{Float64: 0, Valid: true}, false},
		{"negative", "-1", sql.NullFloat64{Float64: -1, Valid: true}, false},
		{"fraction", "0.5", sql.NullFloat64{Float64: 0.5, Valid: true}, false},
		{"exp", "2e1", sql.NullFloat64{Float64: 20, Valid: true}, false},
		{"too_precise", "1e-1337", sql.NullFloat64{Float64: 0, Valid: true}, false},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var actual Float
			if err := actual.UnmarshalText([]byte(st.input)); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, Float{NullFloat64: st.output}, actual)
			}
		})
	}
}

func TestFloat_UnmarshalJSON(t *testing.T) {
	subtests := []struct {
		name   string
		input  string
		output sql.NullFloat64
		error  bool
	}{
		{"null", `null`, sql.NullFloat64{}, false},
		{"bool", `false`, sql.NullFloat64{}, true},
		{"string", `"0"`, sql.NullFloat64{}, true},
		{"too_big", `1e309`, sql.NullFloat64{}, true},
		{"zero", `0`, sql.NullFloat64{Float64: 0, Valid: true}, false},
		{"negative", `-1`, sql.NullFloat64{Float64: -1, Valid: true}, false},
		{"fraction", `0.5`, sql.NullFloat64{Float64: 0.5, Valid: true}, false},
		{"exp", `2e1`, sql.NullFloat64{Float64: 20, Valid: true}, false},
		{"too_precise", `1e-1337`, sql.NullFloat64{Float64: 0, Valid: true}, false},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var actual Float
			if err := actual.UnmarshalJSON([]byte(st.input)); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, Float{NullFloat64: st.output}, actual)
			}
		})
	}
}
