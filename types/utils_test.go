package types

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestName(t *testing.T) {
	subtests := []struct {
		name   string
		input  any
		output string
	}{
		{"nil", nil, "<nil>"},
		{"simple", 1, "int"},
		{"pointer", (*int)(nil), "int"},
		{"package", os.FileMode(0), "FileMode"},
		{"pointer_package", (*fmt.Formatter)(nil), "Formatter"},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			require.Equal(t, st.output, Name(st.input))
		})
	}
}
