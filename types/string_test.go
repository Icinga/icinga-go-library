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

func TestString_UnmarshalText(t *testing.T) {
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
			var actual String

			require.NoError(t, actual.UnmarshalText([]byte(st.io)))
			require.Equal(t, String{NullString: sql.NullString{String: st.io, Valid: true}}, actual)
		})
	}
}

func TestString_UnmarshalJSON(t *testing.T) {
	subtests := []struct {
		name   string
		input  string
		output sql.NullString
		error  bool
	}{
		{"null", `null`, sql.NullString{}, false},
		{"bool", `false`, sql.NullString{}, true},
		{"number", `0`, sql.NullString{}, true},
		{"empty", `""`, sql.NullString{String: "", Valid: true}, false},
		{"nul", `"\u0000"`, sql.NullString{String: "\x00", Valid: true}, false},
		{"space", `" "`, sql.NullString{String: " ", Valid: true}, false},
		{"multiple", `"abc"`, sql.NullString{String: "abc", Valid: true}, false},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var actual String
			if err := actual.UnmarshalJSON([]byte(st.input)); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, String{NullString: st.output}, actual)
			}
		})
	}
}

func TestString_Value(t *testing.T) {
	subtests := []struct {
		name   string
		input  sql.NullString
		output any
	}{
		{"nil", sql.NullString{}, nil},
		{"invalid", sql.NullString{String: "abc"}, nil},
		{"empty", sql.NullString{String: "", Valid: true}, ""},
		{"nul", sql.NullString{String: "\x00", Valid: true}, ""},
		{"space", sql.NullString{String: " ", Valid: true}, " "},
		{"multiple", sql.NullString{String: "abc", Valid: true}, "abc"},
		{"nuls", sql.NullString{String: "\x00 \x00", Valid: true}, " "},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			actual, err := String{st.input}.Value()

			require.NoError(t, err)
			require.Equal(t, st.output, actual)
		})
	}
}
