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
	URL  string `json:"url"`  // URL represents the fully qualified URL to the object in Icinga Web 2.

	// Tags contains additional metadata for the event that uniquely identifies the object it's referring to.
	//
	// It is a map of string keys to string values, allowing for flexible tagging of events if the event
	// name alone is not sufficient to identify the object. In the case of using Icinga DB as a source, the
	// tags will typically look like this:
	// For hosts: {"host": "host_name"} and for services: {"host": "host_name", "service": "service_name"}.
	Tags map[string]string `json:"tags"`

	Type     Type     `json:"type"`               // Type indicates the type of the event (see Type for possible values).
	Severity Severity `json:"severity,omitempty"` // The severity of the event (see Severity for possible values).
	Username string   `json:"username"`           // Username is the name of the user who triggered the event.
	Message  string   `json:"message"`            // Message is a human-readable message describing the event.

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
}
