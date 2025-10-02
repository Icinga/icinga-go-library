package event

import (
	"github.com/icinga/icinga-go-library/types"
)

// Event represents an Icinga Notifications event that can be sent to the Icinga Notifications API.
//
// It contains all the necessary fields to fully describe an Icinga Notifications event and can be used to
// serialize the event to JSON for transmission over HTTP as well as to deserialize it from JSON requests.
type Event struct {
	Name string `json:"name"` // Name is the name of the object this event is all about.

	// URL represents a URL or a relative reference to the object in Icinga Web 2.
	//
	// If the URL field does not contain a URL, but only a reference relative to an Icinga Web URL, the Icinga
	// Notifications daemon will create a URL. This allows a source to set this to something like
	// "/icingadb/host?name=example.com" without having to know the Icinga Web 2 root URL by itself.
	URL string `json:"url"`

	// Tags contains additional metadata for the event that uniquely identifies the object it's referring to.
	//
	// It is a map of string keys to string values, allowing for flexible tagging of events if the event
	// name alone is not sufficient to identify the object. In the case of using Icinga DB as a source, the
	// tags will typically look like this:
	// For hosts: {"host": "host_name"} and for services: {"host": "host_name", "service": "service_name"}.
	Tags map[string]string `json:"tags"`

	// Type indicates the type of the event.
	Type Type `json:"type"`
	// Severity of the event.
	Severity Severity `json:"severity,omitempty"`
	// Username is the name of the user who triggered the event.
	Username string `json:"username"`
	// Message is a human-readable message describing the event.
	Message string `json:"message"`

	// Mute indicates whether the object this event is referring to should be muted or not.
	//
	// If set to true, the object will be muted in Icinga Web 2, meaning that notifications for this object
	// will not be sent out. The MuteReason field can be used to provide a reason for muting the object.
	// If you don't set this field to anything, it will be omitted from the generated JSON.
	Mute types.Bool `json:"mute,omitzero"`

	// MuteReason provides a reason for muting the object if Mute is set to true.
	//
	// Setting this field to an empty string while Mute is true will cause the request to fail,
	// as Icinga Notifications requires a reason for muting an object. Otherwise, it will be omitted
	// from the encoded JSON.
	MuteReason string `json:"mute_reason,omitempty"`

	// RulesVersion and RuleIds are the source rules matching for this Event.
	RulesVersion string   `json:"rules_version"`
	RuleIds      []string `json:"rule_ids"`
}
