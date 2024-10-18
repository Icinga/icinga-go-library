package redis

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStreams_Option(t *testing.T) {
	subtests := []struct {
		name    string
		input   Streams
		outputs [][]string
	}{
		{"nil", nil, [][]string{{}}},
		{"empty", Streams{}, [][]string{{}}},
		{"one", Streams{"key": "id"}, [][]string{{"key", "id"}}},
		{"two", Streams{"key1": "id1", "key2": "id2"}, [][]string{
			{"key1", "key2", "id1", "id2"}, {"key2", "key1", "id2", "id1"},
		}},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			require.Contains(t, st.outputs, st.input.Option())
		})
	}
}
