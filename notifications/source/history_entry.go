//go:generate go tool stringer -linecomment -type NotificationState -output history_entry_string.go

package source

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/icinga/icinga-go-library/types"
)

// NotificationState represents the state of a notification in Icinga notifications.
// It is an integer type with predefined constants for different notification states.
type NotificationState uint8

const (
	NotificationStateNull NotificationState = iota

	NotificationStateSuppressed // suppressed
	NotificationStatePending    // pending
	NotificationStateSent       // sent
	NotificationStateFailed     // failed

	notificationStateMax // internal
)

// Scan implements the sql.Scanner interface.
// Supports SQL NULL.
func (n *NotificationState) Scan(src any) error {
	if src == nil {
		*n = NotificationStateNull
		return nil
	}

	var name string
	switch val := src.(type) {
	case string:
		name = val
	case []byte:
		name = string(val)
	default:
		return fmt.Errorf("unable to scan type %T into NotificationState", src)
	}

	historyType, err := ParseNotificationState(name)
	if err != nil {
		return err
	}

	*n = historyType

	return nil
}

// Value implements the driver.Valuer interface.
// Supports SQL NULL.
func (n NotificationState) Value() (driver.Value, error) {
	if n == NotificationStateNull {
		return nil, nil
	}

	return n.String(), nil
}

// UnmarshalJSON implements the [json.Unmarshaler] interface for NotificationState.
func (n *NotificationState) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*n = NotificationStateNull
		return nil
	}

	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}

	state, err := ParseNotificationState(name)
	if err != nil {
		return err
	}

	*n = state
	return nil
}

// MarshalJSON implements the [json.Marshaler] interface for NotificationState.
func (n NotificationState) MarshalJSON() ([]byte, error) {
	if n != NotificationStateNull {
		return json.Marshal(n.String())
	}
	return json.Marshal(nil)
}

// ParseNotificationState parses a string representation of a notification state and returns the corresponding NotificationState value.
// If the string does not match any known notification state, it returns an error indicating the unknown notification state.
func ParseNotificationState(name string) (NotificationState, error) {
	for s := range notificationStateMax {
		if s != NotificationStateNull && s.String() == name {
			return s, nil
		}
	}

	return NotificationStateNull, fmt.Errorf("unknown notification state %q", name)
}

// NotificationHistory represents a single entry in the notification history retrieved from the Icinga Notifications API.
//
// The struct is designed to be used with JSON serialization and deserialization.
type NotificationHistory struct {
	EventID          types.UUID        `json:"event_id"`
	TriggeredAt      types.UnixMilli   `json:"triggered_at"`
	ContactName      types.String      `json:"contact_name"`
	ContactgroupName types.String      `json:"contactgroup_name"`
	ScheduleName     types.String      `json:"schedule_name"`
	ChannelName      types.String      `json:"channel_name"`
	EventMessage     types.String      `json:"event_message"`
	State            NotificationState `json:"state"`
}
