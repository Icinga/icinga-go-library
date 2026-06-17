// Package event describes the central Icinga Notifications Event type.
package event

import (
	"errors"
	"fmt"

	"github.com/icinga/icinga-go-library/types"
)

// Event represents an Icinga Notifications event that can be sent to the Icinga Notifications API.
//
// It contains all the necessary fields to fully describe an Icinga Notifications event and can be used to
// serialize the event to JSON for transmission over HTTP as well as to deserialize it from JSON requests.
type Event struct {
	// ID is a unique identifier for this specific event.
	//
	// The Name field describes the event, while the ID distinguishes two events that look identical. This is
	// necessary for otherwise identical-looking events. Consider a "door open" event without any additional
	// information. When the source sets the ID to the timestamp when the door was opened, it is possible to
	// distinguish between two consecutive opened doors and the same "door open" event that was submitted twice
	// due to an error.
	//
	// So, when implementing a source, ensure that each event gets its own unique ID. When resubmitting the same
	// event, keep the ID. When submitting a new event - even if the fields are identical - choose a new ID. For
	// the "door open" example, the ID could be "door-open-$TIMESTAMP" or "door-open-$COUNTER".
	ID string `json:"id,omitempty"`

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

	// Severity of the event.
	Severity Severity `json:"severity,omitempty"`
	// Message is a human-readable message describing the event.
	Message string `json:"message"`

	// Muted indicates whether the object this event is referring to is currently muted or not.
	//
	// If set to true, Icinga Notifications will suppress the resulting notifications of a given incident.
	// The MutedReason field must be set if this field is set to both true or false, as it provides the
	// reason for muting or unmuting the incident/object. After the initial event that sets the muted state,
	// the source can omit these fields in follow-up events if the muted state doesn't change, as Icinga
	// Notifications will remember the muted state of the object/incident for subsequent events.
	//
	// Note that setting this field to true while Close is set to true will cause the request to fail.
	// You can use the Validate method to check for these and other invalid combinations of fields before
	// sending the event to the Icinga Notifications API.
	//
	// If this field is not explicitly set by the source, it will be omitted from the encoded JSON.
	Muted types.Bool `json:"muted,omitzero"`

	// MutedReason contains the reason for muting or unmuting the object/incident this event is referring to.
	//
	// This field must be set if the Muted field is set to either true or false, as it provides the reason for
	// muting or unmuting the incident/object. If set to empty string or none at all, it will be omitted from
	// the encoded JSON and the request might fail depending on the value of the Muted field.
	MutedReason string `json:"muted_reason,omitempty"`

	// Incident instructs Icinga Notifications to open a new incident for this event or escalate an existing one.
	//
	// If set, Icinga Notifications will open a new incident for this event if there's none yet,
	// or escalate an existing incident regardless of whether the severity has changed or not.
	//
	// You can only set this field to true or omit it entirely, but not to false. If you set it to false,
	// the request will be rejected by the Icinga Notifications API. You can use the Validate method to
	// check for this and other invalid combinations of fields before sending your request.
	Incident types.Bool `json:"incident,omitzero"`

	// Close instructs Icinga Notifications to close an existing incident for this event.
	//
	// If set, Icinga Notifications will close an existing incident, and depending on the Muted field, it might
	// also trigger notifications and unmute the incident if it was muted before. This option can only be used
	// if the Incident field is set as well, otherwise the request will be rejected by the Icinga Notifications
	// API. You can use the Validate method to check for this and other invalid combinations of fields before
	// sending your request.
	//
	// Like the Incident field, you can only set this field to true or omit it entirely, but not to false.
	Close types.Bool `json:"close,omitzero"`

	// Notify instructs Icinga Notifications to notify the recipients of the incident this event is referring to.
	//
	// Normally, Icinga Notifications will only send notifications for an incident if its severity changes with
	// the ongoing events, but if this field is set to true, it will trigger notifications for the incident regardless
	// of whether the severity has changed or not.
	//
	// If there's no existing incident, this field has no effect. Like the Incident and Close fields, you can
	// only set this field to true or omit it entirely, but not to false. Also, this option cannot be combined
	// with any of the other options other than the Incident and Muted fields.
	Notify types.Bool `json:"notify,omitzero"`

	// CompleteRelations contains a list of relations that should be considered complete for this event.
	//
	// Typically, when Icinga Notifications determines that the event didn't provide complete information
	// needed to evaluate the rules in the [Relations] field, it will instruct the source to provide the
	// missing information in a follow-up request. This field can be used by the source to indicate that
	// it already provided all the available information it has for the certain relations and that Icinga
	// Notifications should not ask for more information for these relations.
	//
	// The relations must be specified as JSONPath expressions that match the structure of the [Relations] field.
	// For example, if the [Relations] field contains a "host" field with some information about the customvars,
	// the source can add `host.vars` or `host.vars.something` to this list to indicate that it already provided
	// all the host's customvars or all the information about the `something` customvar.
	CompleteRelations []string `json:"complete_relations,omitempty"`

	// Relations contains additional information about the relations of the object this event is referring to.
	//
	// This will be used to evaluate JSONPath expressions in the filter columns of the event rules.
	// The structure of this field is flexible and can contain any information that might be relevant
	// for the evaluation of the rules. It will not be used for anything else than evaluating the rules
	// and will not be persisted to the database as well.
	Relations map[string]any `json:"relations,omitempty"`
}

// IsMuted returns true if this event should mute the object/incident it is referring to.
func (e *Event) IsMuted() bool { return e.Muted.Valid && e.Muted.Bool }

// OpenOrEscalate returns true if this event should open an incident or escalate an existing one.
func (e *Event) OpenOrEscalate() bool { return e.Incident.Valid && e.Incident.Bool }

// CloseIncident returns true if this event should close an existing incident.
func (e *Event) CloseIncident() bool { return e.Close.Valid && e.Close.Bool }

// NotifyRecipients returns true if this event should notify the incident recipients.
func (e *Event) NotifyRecipients() bool { return e.Notify.Valid && e.Notify.Bool }

// Validate checks if the event is valid and can be processed by Icinga Notifications.
//
// It checks if the required fields are set and if the values of the fields are valid.
// If any of the checks fail, it returns an error describing the issue. Otherwise, it returns nil.
func (e *Event) Validate() error {
	if len(e.Tags) == 0 {
		return errors.New("invalid event: tags must not be empty")
	}

	for tag, val := range e.Tags {
		if tag == "" {
			return errors.New("invalid event: tag key must not be empty")
		}
		if val == "" {
			return errors.New("invalid event: tag values must not be empty")
		}
		if len(tag) > 255 {
			return fmt.Errorf("invalid event: tag %q is too long, at most 255 chars allowed, %d given", tag, len(tag))
		}
	}

	if !e.OpenOrEscalate() && e.Incident.Valid {
		return errors.New("invalid event: 'incident' can only be set to true or none at all, but not to false")
	}
	if !e.CloseIncident() && e.Close.Valid {
		return errors.New("invalid event: 'close' can only be set to true or none at all, but not to false")
	}
	if !e.NotifyRecipients() && e.Notify.Valid {
		return errors.New("invalid event: 'notify' can only be set to true or none at all, but not to false")
	}

	if !e.OpenOrEscalate() && e.CloseIncident() {
		return errors.New("invalid event: 'close' must not be set if 'incident' is not set")
	}

	if e.Muted.Valid && e.MutedReason == "" {
		return errors.New("invalid event: 'muted_reason' must not be empty if 'muted' is set")
	}
	if e.CloseIncident() && e.IsMuted() {
		return errors.New("invalid event: 'muted' must not be set to true if 'close' is set")
	}

	if !e.OpenOrEscalate() && e.NotifyRecipients() {
		return errors.New("invalid event: 'notify' must not be set if 'incident' is not set")
	}
	if e.CloseIncident() && e.Notify.Valid {
		return errors.New("invalid event: 'notify' must not be set if 'close' is set")
	}

	if !e.OpenOrEscalate() && !e.Muted.Valid {
		return errors.New("invalid event: at least one of 'incident' or 'muted' must be set")
	}

	return nil
}
