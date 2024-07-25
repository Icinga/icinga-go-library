package types

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestUUID_Value(t *testing.T) {
	nonzero := uuid.New()

	subtests := []struct {
		name   string
		input  uuid.UUID
		output []byte
	}{
		{"zero", uuid.UUID{}, make([]byte, 16)},
		{"nonzero", nonzero, nonzero[:]},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			actual, err := UUID{st.input}.Value()

			require.NoError(t, err)
			require.Equal(t, st.output, actual)
		})
	}
}
