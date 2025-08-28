package event

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEvent(t *testing.T) {
	t.Parallel()

	t.Run("JsonEncode", func(t *testing.T) {
		t.Parallel()

		t.Run("Valid Event", func(t *testing.T) {
			t.Parallel()

			event := &Event{
				Name:     "TestEvent",
				URL:      "example.com",
				Tags:     map[string]string{"tag1": "value1"},
				Type:     TypeState,
				Severity: SeverityOK,
				Username: "testuser",
				Message:  "Test",
			}

			data, err := json.Marshal(event)
			require.NoError(t, err)

			expected := `{"name":"TestEvent","url":"example.com","tags":{"tag1":"value1"},"type":"state","severity":"ok","username":"testuser","message":"Test"}`
			assert.Equal(t, expected, string(data))
		})

		t.Run("Empty Severity", func(t *testing.T) {
			t.Parallel()

			event := &Event{
				Name:     "TestEvent",
				URL:      "example.com",
				Tags:     map[string]string{"tag1": "value1"},
				Type:     TypeMute,
				Username: "testuser",
				Message:  "Test",
			}

			data, err := json.Marshal(event)
			require.NoError(t, err)
			assert.NotContains(t, string(data), "\"severity\":", "severity should be omitted when empty")

			event.Severity = SeverityNone
			data, err = json.Marshal(event)
			require.NoError(t, err)
			assert.NotContains(t, string(data), "\"severity\":", "severity should be omitted when set to none")
		})
	})
}
