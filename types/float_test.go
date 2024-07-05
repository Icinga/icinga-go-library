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
