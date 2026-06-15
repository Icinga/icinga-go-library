package event

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/icinga/icinga-go-library/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvent(t *testing.T) {
	t.Parallel()

	t.Run("Validate", func(t *testing.T) {
		t.Parallel()

		assert.ErrorContains(t, (&Event{Tags: map[string]string{"foo": "bar"}}).Validate(), "at least one of 'incident' or 'muted' must be set")

		t.Run("Tags", func(t *testing.T) {
			t.Parallel()

			ev := &Event{Tags: map[string]string{"foo": "bar"}, Incident: types.MakeBool(true)}
			assert.NoError(t, ev.Validate())

			ev.Tags[""] = "foo"
			assert.ErrorContains(t, ev.Validate(), "tag key must not be empty")

			delete(ev.Tags, "")
			ev.Tags["dong"] = ""
			assert.ErrorContains(t, ev.Validate(), "tag values must not be empty")

			delete(ev.Tags, "dong")
			oversized := strings.Repeat("a", 256)
			ev.Tags[oversized] = "oversized"
			assert.ErrorContains(t, ev.Validate(), fmt.Sprintf(`tag %q is too long, at most 255 chars allowed, %d given`, oversized, 256))
		})

		t.Run("Flags", func(t *testing.T) {
			t.Parallel()

			tags := map[string]string{"foo": "bar"}
			mkB := func(v bool) types.Bool { return types.MakeBool(v) }

			t.Run("Muted", func(t *testing.T) {
				t.Parallel()

				assert.NoError(t, (&Event{Tags: tags, Muted: mkB(true), MutedReason: "R"}).Validate())
				assert.NoError(t, (&Event{Tags: tags, Muted: mkB(false), MutedReason: "R"}).Validate())
				assert.ErrorContains(t,
					(&Event{Tags: tags, Muted: mkB(true)}).Validate(),
					"invalid event: 'muted_reason' must not be empty if 'muted' is set")
				assert.ErrorContains(t,
					(&Event{Tags: tags, Muted: mkB(false)}).Validate(),
					"invalid event: 'muted_reason' must not be empty if 'muted' is set")

				assert.NoError(t, (&Event{Tags: tags, Incident: mkB(true), Muted: mkB(true), MutedReason: "R"}).Validate())
				assert.NoError(t, (&Event{Tags: tags, Incident: mkB(true), Muted: mkB(false), MutedReason: "R"}).Validate())
				assert.NoError(t, (&Event{Tags: tags, Incident: mkB(true), Close: mkB(true), Muted: mkB(false), MutedReason: "R"}).Validate())
				assert.ErrorContains(t,
					(&Event{Tags: tags, Incident: mkB(true), Close: mkB(true), Muted: mkB(true), MutedReason: "R"}).Validate(),
					"invalid event: 'muted' must not be set to true if 'close' is set")
			})

			t.Run("Incident", func(t *testing.T) {
				t.Parallel()

				assert.NoError(t, (&Event{Tags: tags, Incident: mkB(true)}).Validate())
				assert.ErrorContains(t, (&Event{Tags: tags, Incident: mkB(false)}).Validate(), "'incident' can only be set to true or none at all")
			})

			t.Run("Close", func(t *testing.T) {
				t.Parallel()

				assert.NoError(t, (&Event{Tags: tags, Incident: mkB(true), Close: mkB(true)}).Validate())
				assert.ErrorContains(t, (&Event{Tags: tags, Close: mkB(false)}).Validate(), "'close' can only be set to true or none at all")
				assert.ErrorContains(t, (&Event{Tags: tags, Close: mkB(true)}).Validate(), "'close' must not be set if 'incident' is not set")
			})

			t.Run("Notify", func(t *testing.T) {
				t.Parallel()

				assert.NoError(t, (&Event{Tags: tags, Incident: mkB(true), Notify: mkB(true)}).Validate())
				assert.NoError(t, (&Event{Tags: tags, Incident: mkB(true), Notify: mkB(true), Muted: mkB(false), MutedReason: "R"}).Validate())
				assert.NoError(t, (&Event{Tags: tags, Incident: mkB(true), Notify: mkB(true), Muted: mkB(true), MutedReason: "R"}).Validate())
				assert.ErrorContains(t, (&Event{Tags: tags, Notify: mkB(false)}).Validate(), "'notify' can only be set to true or none at all")
				assert.ErrorContains(t, (&Event{Tags: tags, Notify: mkB(true)}).Validate(), "'notify' must not be set if 'incident' is not set")
				assert.ErrorContains(t, (&Event{Tags: tags, Incident: mkB(true), Close: mkB(true), Notify: mkB(true)}).Validate(), "'notify' must not be set if 'close' is set")
			})
		})
	})

	t.Run("JsonEncode", func(t *testing.T) {
		t.Parallel()

		t.Run("Valid Event", func(t *testing.T) {
			t.Parallel()

			event := &Event{
				Name:              "TestEvent",
				URL:               "/icingadb/service?name=https%20ssl%20v3.0%20compatibility%20IE%206.0&host.name=example%20host",
				Tags:              map[string]string{"tag1": "value1"},
				Type:              TypeState,
				Severity:          SeverityOK,
				Username:          "testuser",
				Message:           "Test",
				CompleteRelations: []string{"relation1", "relation2"},
				Relations: map[string]any{
					"relation1": "relation1",
					"relation2": "relation2",
				},
			}

			data, err := json.Marshal(event)
			require.NoError(t, err)

			expected := `
				{
					"name":"TestEvent",
					"url":"/icingadb/service?name=https%20ssl%20v3.0%20compatibility%20IE%206.0&host.name=example%20host",
					"tags":{"tag1":"value1"},
					"type":"state",
					"severity":"ok",
					"username":"testuser",
					"message":"Test",
					"complete_relations":["relation1", "relation2"],
					"relations":{"relation1":"relation1","relation2":"relation2"}
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
