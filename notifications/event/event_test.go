package event

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvent(t *testing.T) {
	t.Parallel()

	t.Run("JsonEncode", func(t *testing.T) {
		t.Parallel()

		t.Run("Valid Event", func(t *testing.T) {
			t.Parallel()

			event := &Event{
				Name:         "TestEvent",
				URL:          "example.com",
				Tags:         map[string]string{"tag1": "value1"},
				Type:         TypeState,
				Severity:     SeverityOK,
				Username:     "testuser",
				Message:      "Test",
				RulesVersion: "0x1",
				RuleIds:      []int64{1, 2, 3, 6},
			}

			data, err := json.Marshal(event)
			require.NoError(t, err)

			expected := `
				{
					"name":"TestEvent",
					"url":"example.com",
					"tags":{"tag1":"value1"},
					"type":"state",
					"severity":"ok",
					"username":"testuser",
					"message":"Test",
					"rules_version": "0x1",
					"rule_ids": [1, 2, 3, 6]
				}`
			assert.JSONEq(t, expected, string(data), "JSON encoding does not match expected output")
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
