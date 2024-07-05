package types

import (
	"database/sql"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestInt_MarshalJSON(t *testing.T) {
	subtests := []struct {
		name   string
		input  sql.NullInt64
		output string
	}{
		{"null", sql.NullInt64{}, `null`},
		{"invalid", sql.NullInt64{Int64: 42}, `null`},
		{"zero", sql.NullInt64{Int64: 0, Valid: true}, `0`},
		{"negative", sql.NullInt64{Int64: -1, Valid: true}, `-1`},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			actual, err := Int{st.input}.MarshalJSON()

			require.NoError(t, err)
			require.Equal(t, st.output, string(actual))
		})
	}
}
