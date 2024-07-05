package types

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
	"unicode/utf8"
)

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
