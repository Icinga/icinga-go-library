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
				URL:          "/icingadb/service?name=https%20ssl%20v3.0%20compatibility%20IE%206.0&host.name=example%20host",
				Tags:         map[string]string{"tag1": "value1"},
				ExtraTags:    map[string]string{},
				Type:         TypeState,
				Severity:     SeverityOK,
				Username:     "testuser",
				Message:      "Test",
				RulesVersion: "0x1",
				RuleIds:      []string{"1", "2", "3", "6"},
			}

			data, err := json.Marshal(event)
			require.NoError(t, err)

			expected := `
				{
					"name":"TestEvent",
					"url":"/icingadb/service?name=https%20ssl%20v3.0%20compatibility%20IE%206.0&host.name=example%20host",
					"tags":{"tag1":"value1"},
					"extra_tags":{},
					"type":"state",
					"severity":"ok",
					"username":"testuser",
					"message":"Test",
					"rules_version": "0x1",
					"rule_ids": ["1", "2", "3", "6"]
				}`
			assert.JSONEq(t, expected, string(data), "JSON encoding does not match expected output")
		})

		t.Run("Empty Severity", func(t *testing.T) {
			t.Parallel()

			event := &Event{
				Name:     "TestEvent",
				URL:      "https://example.com/icingaweb2/icingadb/service?name=https%20ssl%20v3.0%20compatibility%20IE%206.0&host.name=example%20host",
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
