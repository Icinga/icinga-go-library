package types

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestStringList(t *testing.T) {
	t.Parallel()

	t.Run("MakeStringList", func(t *testing.T) {
		t.Parallel()

		subtests := []struct {
			name         string
			input        []string
			transformers []func(list *StringList)
			output       StringList
		}{
			{
				name:   "empty",
				input:  []string{},
				output: StringList{List: []string{}, Valid: true},
			},
			{
				name:   "null",
				input:  nil,
				output: StringList{Valid: true},
			},
			{
				name:   "space",
				input:  []string{" "},
				output: StringList{List: []string{" "}, Valid: true},
			},
			{
				name:   "valid-text",
				input:  []string{"abc"},
				output: StringList{List: []string{"abc"}, Valid: true},
			},
			{
				name:         "empty-transform-empty-to-null",
				input:        []string{},
				transformers: []func(*StringList){TransformEmptyStringListToNull},
				output:       StringList{List: []string{}, Valid: false},
			},
			{
				name:         "empty-transform-empty-to-null",
				input:        nil,
				transformers: []func(*StringList){TransformEmptyStringListToNull},
				output:       StringList{Valid: false},
			},
			{
				name:         "valid-text-transform-empty-to-null",
				input:        []string{"abc"},
				transformers: []func(list *StringList){TransformEmptyStringListToNull},
				output:       StringList{List: []string{"abc"}, Valid: true},
			},
		}

		for _, st := range subtests {
			t.Run(st.name, func(t *testing.T) {
				t.Parallel()

				require.Equal(t, st.output, MakeStringList(st.input, st.transformers...))
			})
		}
	})

	t.Run("MarshalJSON", func(t *testing.T) {
		t.Parallel()

		subtests := []struct {
			name   string
			input  StringList
			output string
		}{
			{"null", StringList{}, `null`},
			{"invalid", StringList{List: []string{"foo"}}, `null`},
			{"valid", StringList{List: []string{"foo"}, Valid: true}, `["foo"]`},
			{"empty", StringList{List: []string{}, Valid: true}, `[]`},
			{"multiple", StringList{List: []string{"foo", "bar"}, Valid: true}, `["foo","bar"]`},
		}

		for _, st := range subtests {
			t.Run(st.name, func(t *testing.T) {
				t.Parallel()

				actual, err := st.input.MarshalJSON()

				require.NoError(t, err)
				require.True(t, utf8.Valid(actual))
				require.Equal(t, st.output, string(actual))
			})
		}
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		t.Parallel()

		subtests := []struct {
			name   string
			input  string
			output StringList
			error  bool
		}{
			{"null", `null`, StringList{}, false},
			{"bool", `false`, StringList{}, true},
			{"number", `0`, StringList{}, true},
			{"empty", `[]`, StringList{List: []string{}, Valid: true}, false},
			{"multiple", `["foo","bar"]`, StringList{List: []string{"foo", "bar"}, Valid: true}, false},
			{"syntax_error", `["foo","bar",]`, StringList{List: []string{"", ""}, Valid: true}, true},
		}

		for _, st := range subtests {
			t.Run(st.name, func(t *testing.T) {
				t.Parallel()

				var actual StringList
				if err := actual.UnmarshalJSON([]byte(st.input)); st.error {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					require.Equal(t, st.output, actual)
				}
			})
		}
	})

	t.Run("Value", func(t *testing.T) {
		t.Parallel()

		subtests := []struct {
			name   string
			input  StringList
			output any
		}{
			{"empty", MakeStringList([]string{}), []string{}},
			{"empty_nulled", MakeStringList([]string{}, TransformEmptyStringListToNull), nil},
			{"nil", MakeStringList(nil), []string(nil)},
			{"nil_nulled", MakeStringList(nil, TransformEmptyStringListToNull), nil},
			{"invalid", MakeStringList([]string{"abc"}), []string{"abc"}},
		}

		for _, st := range subtests {
			t.Run(st.name, func(t *testing.T) {
				actual, err := st.input.Value()

				require.NoError(t, err)
				require.Equal(t, st.output, actual)
			})
		}
	})
}
