package types

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUUID_Value(t *testing.T) {
	nonzero := uuid.New()

	subtests := []struct {
		name  string
		input uuid.UUID
	}{
		{"zero", uuid.UUID{}},
		{"nonzero", nonzero},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			actual, err := MakeUUID(st.input, TransformNilUUIDToNull).Value()
			require.NoError(t, err)
			if st.name == "zero" {
				require.Nil(t, actual)
			} else {
				require.Equal(t, st.input[:], actual)
			}
		})
	}
}
