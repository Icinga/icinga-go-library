package redis

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStreams_Option(t *testing.T) {
	subtests := []struct {
		name   string
		input  Streams
		output []string
	}{
		{"nil", nil, []string{}},
		{"empty", Streams{}, []string{}},
		{"one", Streams{"key": "id"}, []string{"key", "id"}},
		{"two", Streams{"key1": "id1", "key2": "id2"}, []string{"key1", "key2", "id1", "id2"}},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			require.Equal(t, st.output, st.input.Option())
		})
	}
}
