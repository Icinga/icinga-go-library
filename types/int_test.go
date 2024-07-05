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

func TestInt_UnmarshalText(t *testing.T) {
	subtests := []struct {
		name   string
		input  string
		output sql.NullInt64
		error  bool
	}{
		{"empty", "", sql.NullInt64{}, true},
		{"2p64", "18446744073709551616", sql.NullInt64{}, true},
		{"float", "0.0", sql.NullInt64{}, true},
		{"zero", "0", sql.NullInt64{Int64: 0, Valid: true}, false},
		{"negative", "-1", sql.NullInt64{Int64: -1, Valid: true}, false},
		{"2p62", "4611686018427387904", sql.NullInt64{Int64: 1 << 62, Valid: true}, false},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var actual Int
			if err := actual.UnmarshalText([]byte(st.input)); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, Int{NullInt64: st.output}, actual)
			}
		})
	}
}
