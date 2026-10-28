// Package event describes the central Icinga Notifications Event type.
package event

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/icinga/icinga-go-library/types"
)

// MaxSummaryLen is the maximum rune length of Event.Summary.
const MaxSummaryLen = 1024

// Event represents an Icinga Notifications event that can be sent to the Icinga Notifications API.
//
// It contains all the necessary fields to fully describe an Icinga Notifications event and can be used to
// serialize the event to JSON for transmission over HTTP as well as to deserialize it from JSON requests.
//
// Please refer to the [Icinga Notifications API documentation] for more information on the behavior of the
// fields and how they affect the incident state and notifications. Also, please use [Event.Validate] to check
// for invalid combinations of fields before sending the event to Icinga Notifications.
//
// [Icinga Notifications API documentation]: https://icinga.com/docs/icinga-notifications/latest/doc/20-HTTP-API/#process-event
type Event struct {
	// ID is a globally unique identifier [types.UUID] for the event.
	//
	// While Tags identify the object an event refers to, ID identifies the event itself. Icinga Notifications
	// uses it to detect duplicates: an event whose ID has already been processed is treated as a resubmission
	// of that event. Without an ID, a "door open" event that was sent twice due to a network error would be
	// indistinguishable from the door being opened twice.
	//
	// So, a source should generate a new ID for every event it emits, even if all other fields match a previous
	// event, and reuse an ID only when resubmitting the very same event. If omitted, Icinga Notifications derives
	// the ID from the event's content, so identical events without an ID are treated as duplicates of each other.
	ID types.UUID `json:"id,omitzero"`

	// Name is the human-readable display name of the object, e.g. a host or service display name.
	//
	// It is shown in notifications and in Icinga Notifications Web. It must not be empty, but it
	// doesn't need to be unique.
	Name string `json:"name"`

	// URL points to the object this event refers to, e.g. its detail view in the source's web interface.
	//
	// It is passed on to the notified contacts, so it has to be an absolute URL such as
	// "https://example.com/icingaweb2/icingadb/host?name=example". Icinga Notifications does not know
	// where the source's web interface lives and therefore rejects a relative reference. This field may be
	// left empty if the object has no web interface to link to.
	URL string `json:"url"`

	// Tags uniquely identifies the object this event refers to.
	//
	// It is a map of string keys to string values that together form the identity of the object, whereas Name
	// is only used for display purposes. It must not be empty, and neither its keys nor its values may be empty.
	// Keys must be at most 255 characters long. In the case of using Icinga DB as a source, the tags will
	// typically look like this:
	//
	//	host:    {"environment": "environment_id", "host": "host_name"}
	//	service: {"environment": "environment_id", "host": "host_name", "service": "service_name"}
	Tags map[string]string `json:"tags"`

	// Severity indicates the [Severity] of the event, i.e. "ok", "warning", "error" or "critical".
	//
	// This must be set if and only if Incident is set to true. The zero value [SeverityNone] means
	// unset and is omitted from the encoded JSON.
	Severity Severity `json:"severity,omitempty"`

	// Summary describes this event in a single line.
	//
	// It will be used in situations where space is limited, such as email subject lines.
	// The full text goes into Message.
	//
	// It must not be empty, must not contain any line breaks, and must be at most [MaxSummaryLen] characters long.
	Summary string `json:"summary"`

	// Message is the complete human-readable message of the event.
	//
	// It is optional and may span multiple lines.
	Message string `json:"message"`

	// Type of the event, e.g. "downtime_start", "downtime_end", etc.
	//
	// This must be set if and only if Incident is set to false.
	Type string `json:"type,omitempty"`

	// Muted indicates whether the object this event is referring to is currently muted or not.
	//
	// If set to true, Icinga Notifications will suppress the resulting notifications of a given incident.
	// If set to false, the object/incident is unmuted. MutedReason must be set whenever this field is set
	// (true or false), as it provides the reason for muting or unmuting the incident/object. After the initial
	// event that sets the muted state, the source can omit both Muted and MutedReason in follow-up events if the
	// muted state doesn't change, as Icinga Notifications will remember the muted state of the object/incident.
	//
	// An event may omit Incident and set only Muted (and MutedReason), in which case it merely updates the muted
	// state of the object/incident. However, this field must not be combined with Incident set to false, and
	// it must not be set to true if Close is set.
	//
	// If not explicitly set by the source, it will be omitted from the encoded JSON.
	Muted types.Bool `json:"muted,omitzero"`

	// MutedReason contains the reason for muting or unmuting the object/incident this event is referring to.
	//
	// This must be non-empty whenever Muted is set (true or false).
	MutedReason string `json:"muted_reason,omitempty"`

	// Incident controls whether this event affects an incident (true) or is a notification-only event (false).
	//
	// If set to true, Icinga Notifications will open a new incident for this event if there's none yet, or trigger
	// new escalations for an existing incident depending on the severity of the event. Severity must be set then.
	// See [Event.OpenOrEscalate].
	//
	// If set to false, Icinga Notifications treats this event as a notification-only/informational event. In this
	// case, Type must be provided to describe the kind of event this is, e.g. "downtime_start", etc., while Severity,
	// Muted, Close and Notify must not be set. Such events are not tied to an incident's state and do not trigger
	// escalations, but recipients of an existing incident may still be notified. See [Event.NotificationOnly].
	//
	// If omitted, Muted must be set, and the event only updates the muted state of the object/incident.
	Incident types.Bool `json:"incident,omitzero"`

	// Close instructs Icinga Notifications to close an existing incident for this event.
	//
	// If set, Icinga Notifications will close an existing incident. When combined with Muted set to false, it also
	// unmutes the incident if it was muted before, which might trigger notifications as well. Muted set to true is
	// not allowed, though, as closing and muting an incident at the same time would be contradictory.
	//
	// This field can only be used if Incident is set to true as well, otherwise the request will be rejected by
	// Icinga Notifications. If there's no existing incident, Icinga Notifications will create a new one and close
	// it immediately.
	//
	// You can only set this field to true or omit it entirely. It must not be combined with Notify.
	Close types.Bool `json:"close,omitzero"`

	// Notify instructs Icinga Notifications to notify the recipients of the incident this event is referring to.
	//
	// Normally, Icinga Notifications will only send notifications for an incident if its severity changes with the
	// ongoing event, or new escalations are triggered. However, you can use this field to bypass this behavior and
	// force Icinga Notifications to notify all existing recipients of the incident. For example, Icinga DB uses this
	// for volatile checks to trigger notifications with each check result, even if the severity doesn't change.
	// If there's no existing incident, this field has no effect.
	//
	// You can only set this field to true or omit it entirely. It requires Incident to be true and must
	// not be combined with Close.
	Notify types.Bool `json:"notify,omitzero"`

	// CompleteRelations contains a list of relations that should be considered complete for this event.
	//
	// Typically, when Icinga Notifications determines that the event didn't provide complete information in
	// [Event.Relations] that is needed to evaluate the rules, it will instruct the source to provide the missing
	// information in a follow-up request. This field can be used by the source to indicate that it already provided
	// all the available information it has for certain relations and that Icinga Notifications should not ask for
	// more information for these relations.
	//
	// The relations must be specified as JSONPath expressions that match the structure of [Event.Relations].
	// For example, if [Event.Relations] contains a "host" field with the host's custom variables, the source
	// can add `host.vars` or `host.vars.something` to this list to indicate that it already provided all the
	// host's custom variables or all the information about the `something` custom variable. The leading `$.`
	// (or just `$` for bracket notation, e.g. `$["host"]`) is optional, as Icinga Notifications prepends it
	// if missing.
	CompleteRelations []string `json:"complete_relations,omitempty"`

	// Relations contains additional information about the relations of the object this event is referring to.
	//
	// This will be used to evaluate JSONPath expressions in the filter expressions of event rules.
	// The structure of this field is flexible and can contain any information that might be relevant
	// for the evaluation of the rules. It will not be used for anything other than evaluating the rules.
	Relations map[string]any `json:"relations,omitempty"`
}

// IsMuted returns true if this event should mute the object/incident it is referring to.
func (e *Event) IsMuted() bool { return e.Muted.Valid && e.Muted.Bool }

// OpenOrEscalate returns true if this event should open an incident or escalate an existing one.
func (e *Event) OpenOrEscalate() bool { return e.Incident.Valid && e.Incident.Bool }

// NotificationOnly returns true if this event should notify without changing the incident state.
func (e *Event) NotificationOnly() bool { return e.Incident.Valid && !e.Incident.Bool }

// CloseIncident returns true if this event should close an existing incident.
func (e *Event) CloseIncident() bool { return e.Close.Valid && e.Close.Bool }

// NotifyRecipients returns true if this event should notify the incident recipients.
func (e *Event) NotifyRecipients() bool { return e.Notify.Valid && e.Notify.Bool }

// Validate checks if the event is valid and can be processed by Icinga Notifications.
//
// It checks if the required fields are set and if the values of the fields are valid.
// If any of the checks fail, it returns an error describing the issue. Otherwise, it returns nil.
func (e *Event) Validate() error {
	if e.Name == "" {
		return errors.New("invalid event: 'name' must not be empty")
	}

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
		if l := utf8.RuneCountInString(tag); l > 255 {
			return fmt.Errorf("invalid event: tag %q is too long, at most 255 chars allowed, %d given", tag, l)
		}
	}

	if l := utf8.RuneCountInString(e.Summary); l == 0 {
		return errors.New("invalid event: summary must not be empty")
	} else if l > MaxSummaryLen {
		return fmt.Errorf("invalid event: summary must be at most %d chars, %d given", MaxSummaryLen, l)
	}
	if strings.ContainsAny(e.Summary, "\r\n") {
		return errors.New("invalid event: summary contains newline")
	}

	// A source has to submit an absolute URL, as Icinga Notifications does not know where the source's web
	// interface lives and can therefore not complete a relative reference for the notified contacts. An empty
	// URL is still okay, it just means that this object has no web interface to link to.
	if e.URL != "" {
		u, err := url.Parse(e.URL)
		if err != nil {
			return fmt.Errorf("invalid event: url is not a valid URL: %w", err)
		}

		if !u.IsAbs() {
			return fmt.Errorf("invalid event: url must be an absolute URL, got %q", e.URL)
		}
	}

	if e.OpenOrEscalate() != (e.Severity != SeverityNone) {
		return errors.New("invalid event: 'severity' must be set if and only if 'incident' is set to true")
	}

	if !e.CloseIncident() && e.Close.Valid {
		return errors.New("invalid event: 'close' can only be set to true or none at all, but not to false")
	}
	if !e.NotifyRecipients() && e.Notify.Valid {
		return errors.New("invalid event: 'notify' can only be set to true or none at all, but not to false")
	}

	if !e.OpenOrEscalate() && e.CloseIncident() {
		return errors.New("invalid event: 'close' must not be set if 'incident' is not set to true")
	}

	if e.Muted.Valid && e.MutedReason == "" {
		return errors.New("invalid event: 'muted_reason' must not be empty if 'muted' is set")
	}
	if e.CloseIncident() && e.IsMuted() {
		return errors.New("invalid event: 'muted' must not be set to true if 'close' is set")
	}

	if !e.OpenOrEscalate() && e.NotifyRecipients() {
		return errors.New("invalid event: 'notify' must not be set if 'incident' is not set to true")
	}
	if e.CloseIncident() && e.Notify.Valid {
		return errors.New("invalid event: 'notify' must not be set if 'close' is set")
	}

	if !e.Incident.Valid && !e.Muted.Valid {
		return errors.New("invalid event: at least one of 'incident' or 'muted' must be set")
	}

	if e.NotificationOnly() && e.Muted.Valid {
		return errors.New("invalid event: 'incident' must not be set to false if 'muted' is set")
	}

	if e.NotificationOnly() && e.Type == "" {
		return errors.New("invalid event: 'type' must be set if 'incident' is set to false")
	}
	if e.Type != "" && !e.NotificationOnly() {
		return errors.New("invalid event: 'type' must not be set if `incident` is not set to false")
	}

	return nil
}
