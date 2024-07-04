package types

import (
	"database/sql"
	"github.com/stretchr/testify/require"
	"testing"
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
