package event

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
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

		t.Run("URL", func(t *testing.T) {
			t.Parallel()

			for _, tc := range []struct {
				name    string
				url     string
				isValid bool
			}{
				{name: "Absolute URL", url: "https://example.com/icingaweb2/icingadb/host?name=example", isValid: true},
				{name: "Absolute URL of unknown scheme", url: "icinga://example.com/host/example", isValid: true},
				{name: "Empty URL", url: "", isValid: true},
				{name: "Relative URL with leading slash", url: "/icingadb/host?name=example", isValid: false},
				{name: "Relative URL without leading slash", url: "icingadb/host?name=example", isValid: false},
				{name: "Protocol relative URL", url: "//example.com/icingadb/host?name=example", isValid: false},
				{name: "Invalid URL", url: "http://[invalid-url", isValid: false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()

					ev := &Event{
						URL:      tc.url,
						Tags:     map[string]string{"foo": "bar"},
						Incident: types.MakeBool(true),
					}

					if tc.isValid {
						assert.NoError(t, ev.Validate())
					} else {
						assert.ErrorContains(t, ev.Validate(), "invalid event: url")
					}
				})
			}
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
	t.Run("JsonDecode", func(t *testing.T) {
		t.Parallel()

		assertFunc := func(t *testing.T, event Event) {
			assert.Equal(t, "TestEvent", event.Name)
			assert.Equal(t, "/icingadb/service?name=https%20ssl%20v3.0%20compatibility%20IE%206.0&host.name=example%20host", event.URL)
			assert.Equal(t, map[string]string{"tag1": "value1"}, event.Tags)
			assert.Equal(t, SeverityOK, event.Severity)
			assert.Equal(t, "Test", event.Message)
			assert.Equal(t, []string{"relation1", "relation2"}, event.CompleteRelations)
			assert.Equal(t, map[string]any{"relation1": "relation1", "relation2": "relation2"}, event.Relations)
		}

		t.Run("Valid Event", func(t *testing.T) {
			t.Parallel()

			data := `
				{
					"id":"2e9dd0b2-9555-4627-871e-c8696e742486",
					"name":"TestEvent",
					"url":"/icingadb/service?name=https%20ssl%20v3.0%20compatibility%20IE%206.0&host.name=example%20host",
					"tags":{"tag1":"value1"},
					"severity":"ok",
					"message":"Test",
					"complete_relations":["relation1", "relation2"],
					"relations":{"relation1":"relation1","relation2":"relation2"}
				}`
			var event Event
			require.NoError(t, json.Unmarshal([]byte(data), &event))

			parsedUUID, err := uuid.Parse("2e9dd0b2-9555-4627-871e-c8696e742486")
			require.NoError(t, err, "Failed to parse UUID")

			assert.Equal(t, types.MakeUUID(parsedUUID), event.ID)
			assertFunc(t, event)
		})

		t.Run("Valid Event Without UUID", func(t *testing.T) {
			t.Parallel()

			data := `
				{
					"name":"TestEvent",
					"url":"/icingadb/service?name=https%20ssl%20v3.0%20compatibility%20IE%206.0&host.name=example%20host",
					"tags":{"tag1":"value1"},
					"severity":"ok",
					"message":"Test",
					"complete_relations":["relation1", "relation2"],
					"relations":{"relation1":"relation1","relation2":"relation2"}
				}`
			var event Event
			require.NoError(t, json.Unmarshal([]byte(data), &event))

			assert.Equal(t, types.UUID{}, event.ID, "Expected empty UUID when not provided in JSON")
			assertFunc(t, event)
		})

		t.Run("Invalid Event", func(t *testing.T) {
			t.Parallel()

			data := `
				{
					"id":"invalid-uuid",
					"name":"TestEvent",
					"url":"/icingadb/service?name=https%20ssl%20v3.0%20compatibility%20IE%206.0&host.name=example%20host",
					"tags":{"tag1":"value1"},
					"severity":"ok",
					"message":"Test",
					"complete_relations":["relation1", "relation2"],
					"relations":{"relation1":"relation1","relation2":"relation2"}
				}`
			var event Event
			err := json.Unmarshal([]byte(data), &event)
			assert.Error(t, err, "Expected error when unmarshalling invalid UUID")
		})
	})

	t.Run("JsonEncode", func(t *testing.T) {
		t.Parallel()

		t.Run("Valid Event", func(t *testing.T) {
			t.Parallel()

			originalUUID, err := uuid.Parse("2e9dd0b2-9555-4627-871e-c8696e742486")
			require.NoError(t, err, "Failed to parse UUID")

			event := &Event{
				ID:                types.MakeUUID(originalUUID),
				Name:              "TestEvent",
				URL:               "https://example.com/icingaweb2/icingadb/service?name=https%20ssl%20v3.0%20compatibility%20IE%206.0&host.name=example%20host",
				Tags:              map[string]string{"tag1": "value1"},
				Severity:          SeverityOK,
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
					"id":"2e9dd0b2-9555-4627-871e-c8696e742486",
					"name":"TestEvent",
					"url":"https://example.com/icingaweb2/icingadb/service?name=https%20ssl%20v3.0%20compatibility%20IE%206.0&host.name=example%20host",
					"tags":{"tag1":"value1"},
					"severity":"ok",
					"message":"Test",
					"complete_relations":["relation1", "relation2"],
					"relations":{"relation1":"relation1","relation2":"relation2"}
				}`
			assert.JSONEq(t, expected, string(data), "JSON encoding does not match expected output")
		})

		t.Run("Valid Event Without UUID", func(t *testing.T) {
			t.Parallel()

			event := &Event{Tags: map[string]string{"tag1": "value1"}}
			data, err := json.Marshal(event)
			require.NoError(t, err)

			expected := `
			   {
				  "name":"",
				  "message":"",
				  "url":"",
				  "tags":{"tag1":"value1"}
			   }`
			assert.JSONEq(t, expected, string(data), "JSON encoding does not match expected output")
		})

		t.Run("Empty Severity", func(t *testing.T) {
			t.Parallel()

			event := &Event{
				Name:    "TestEvent",
				URL:     "https://example.com/icingaweb2/icingadb/service?name=https%20ssl%20v3.0%20compatibility%20IE%206.0&host.name=example%20host",
				Tags:    map[string]string{"tag1": "value1"},
				Message: "Test",
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
