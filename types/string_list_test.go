package types

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestMakeStringList(t *testing.T) {
	subtests := []struct {
		name         string
		input        []string
		transformers []func(list *StringList)
		output       NullList[string]
	}{
		{
			name:   "empty",
			input:  []string{},
			output: NullList[string]{Elements: []string{}, Valid: true},
		},
		{
			name:   "nul",
			input:  nil,
			output: NullList[string]{Valid: true},
		},
		{
			name:   "space",
			input:  []string{" "},
			output: NullList[string]{Elements: []string{" "}, Valid: true},
		},
		{
			name:   "valid-text",
			input:  []string{"abc"},
			output: NullList[string]{Elements: []string{"abc"}, Valid: true},
		},
		{
			name:         "empty-transform-empty-to-null",
			input:        []string{},
			transformers: []func(*StringList){TransformEmptyStringListToNull},
			output:       NullList[string]{Elements: []string{}, Valid: false},
		},
		{
			name:         "empty-transform-empty-to-null",
			input:        nil,
			transformers: []func(*StringList){TransformNilStringListToNull},
			output:       NullList[string]{Valid: false},
		},
		{
			name:         "valid-text-transform-empty-to-null",
			input:        []string{"abc"},
			transformers: []func(list *StringList){TransformEmptyStringListToNull},
			output:       NullList[string]{Elements: []string{"abc"}, Valid: true},
		},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			require.Equal(t, StringList{st.output}, MakeStringList(st.input, st.transformers...))
		})
	}
}

func TestStringList_MarshalJSON(t *testing.T) {
	subtests := []struct {
		name   string
		input  NullList[string]
		output string
	}{
		{"null", NullList[string]{}, `null`},
		{"invalid", NullList[string]{Elements: []string{"foo"}}, `null`},
		{"valid", NullList[string]{Elements: []string{"foo"}, Valid: true}, `["foo"]`},
		{"empty", NullList[string]{Elements: []string{}, Valid: true}, `[]`},
		{"nul", NullList[string]{Valid: true}, `null`},
		{"multiple", NullList[string]{Elements: []string{"foo", "bar"}, Valid: true}, `["foo","bar"]`},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			actual, err := StringList{st.input}.MarshalJSON()

			require.NoError(t, err)
			require.True(t, utf8.Valid(actual))
			require.Equal(t, st.output, string(actual))
		})
	}
}

//func TestStringList_UnmarshalText(t *testing.T) {
//	subtests := []struct {
//		name   string
//		input  string
//		output []string
//	}{
//		{"empty", `[""]`, []string{""}},
//		{"nul", `["\x00"]`, []string{"\x00"}},
//		{"space", `[" "]`, []string{" "}},
//		{"multiple", `["foo"]`, []string{"foo"}},
//	}
//
//	for _, st := range subtests {
//		t.Run(st.name, func(t *testing.T) {
//			var actual StringList
//
//			require.NoError(t, actual.UnmarshalText([]byte(st.input)))
//			require.Equal(t, StringList{Elements: st.output, Valid: true}, actual)
//		})
//	}
//}

func TestStringList_UnmarshalJSON(t *testing.T) {
	subtests := []struct {
		name   string
		input  string
		output NullList[string]
		error  bool
	}{
		{"null", `null`, NullList[string]{}, false},
		{"bool", `false`, NullList[string]{}, true},
		{"number", `0`, NullList[string]{}, true},
		{"empty", `[]`, NullList[string]{Elements: []string{}, Valid: true}, false},
		{"multiple", `["foo","bar"]`, NullList[string]{Elements: []string{"foo", "bar"}, Valid: true}, false},
		{"syntax_error", `["foo","bar",]`, NullList[string]{Elements: []string{"", ""}, Valid: true}, true},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var actual StringList
			if err := actual.UnmarshalJSON([]byte(st.input)); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, StringList{st.output}, actual)
			}
		})
	}
}

func TestStringList_Value(t *testing.T) {
	subtests := []struct {
		name   string
		input  NullList[string]
		output any
	}{
		{"nil", NullList[string]{}, nil},
		{"invalid", NullList[string]{Elements: []string{"abc"}}, nil},
		{"empty", NullList[string]{Elements: []string{}, Valid: true}, []string{}},
		{"nul", NullList[string]{Valid: true}, []string(nil)},
		//{"space", NullList[string]{Elements: []string{" "}, Valid: true}, " "},
		//{"multiple", NullList[string]{Elements: []string{"abc"}, Valid: true}, "abc"},
		//{"nuls", NullList[string]{Elements: []string{"\x00 \x00"}, Valid: true}, " "},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			actual, err := StringList{st.input}.Value()

			require.NoError(t, err)
			require.Equal(t, st.output, actual)
		})
	}
}
