package types

import (
	"database/sql"
	"github.com/stretchr/testify/require"
	"testing"
	"unicode/utf8"
)

func TestMakeString(t *testing.T) {
	subtests := []struct {
		name string
		io   string
	}{
		{"empty", ""},
		{"nul", "\x00"},
		{"space", " "},
		{"multiple", "abc"},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			require.Equal(t, String{NullString: sql.NullString{String: st.io, Valid: true}}, MakeString(st.io))
		})
	}
}

func TestString_MarshalJSON(t *testing.T) {
	subtests := []struct {
		name   string
		input  sql.NullString
		output string
	}{
		{"null", sql.NullString{}, `null`},
		{"invalid", sql.NullString{String: "abc"}, `null`},
		{"empty", sql.NullString{String: "", Valid: true}, `""`},
		{"nul", sql.NullString{String: "\x00", Valid: true}, `"\u0000"`},
		{"space", sql.NullString{String: " ", Valid: true}, `" "`},
		{"multiple", sql.NullString{String: "abc", Valid: true}, `"abc"`},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			actual, err := String{st.input}.MarshalJSON()

			require.NoError(t, err)
			require.True(t, utf8.Valid(actual))
			require.Equal(t, st.output, string(actual))
		})
	}
}
