package event

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestType(t *testing.T) {
	t.Parallel()

	t.Run("MarshalJSON", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			typeVal  Type
			expected string
		}{
			{"Unknown", TypeUnknown, "null"},
			{"State", TypeState, `"state"`},
			{"Mute", TypeMute, `"mute"`},
			{"Unmute", TypeUnmute, `"unmute"`},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				data, err := tt.typeVal.MarshalJSON()
				require.NoError(t, err)
				assert.Equal(t, tt.expected, string(data))
			})
		}
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			input    string
			expected Type
			err      bool
		}{
			{"Unknown", "null", TypeUnknown, false},
			{"State", `"state"`, TypeState, false},
			{"Mute", `"mute"`, TypeMute, false},
			{"Unmute", `"unmute"`, TypeUnmute, false},
			{"Invalid", `"invalid"`, TypeUnknown, true}, // Should return an error for unsupported type.
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var tType Type
				if err := tType.UnmarshalJSON([]byte(tt.input)); tt.err {
					assert.Error(t, err)
				} else {
					require.NoError(t, err)
					assert.Equal(t, tt.expected, tType)
				}
			})
		}
	})

	t.Run("Scan", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			input    any
			expected Type
			err      bool
		}{
			{"Unknown", nil, TypeUnknown, false},
			{"State", "state", TypeState, false},
			{"Mute", "mute", TypeMute, false},
			{"Unmute", "unmute", TypeUnmute, false},
			{"Invalid", "invalid", TypeUnknown, true}, // Should return an error for unsupported type.
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var tType Type
				if err := tType.Scan(tt.input); tt.err {
					assert.Error(t, err)
				} else {
					require.NoError(t, err)
					assert.Equal(t, tt.expected, tType)
				}
			})
		}
	})

	t.Run("Value", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			typeVal  Type
			expected any
		}{
			{"Unknown", TypeUnknown, nil},
			{"State", TypeState, "state"},
			{"Mute", TypeMute, "mute"},
			{"Unmute", TypeUnmute, "unmute"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				value, err := tt.typeVal.Value()
				require.NoError(t, err)
				assert.Equal(t, tt.expected, value)
			})
		}
	})
}
