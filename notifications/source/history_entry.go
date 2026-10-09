package source

import (
	"github.com/icinga/icinga-go-library/types"
)

// NotificationHistory represents a single entry in the notification history retrieved from the Icinga Notifications API.
//
// The struct is designed to be used with JSON serialization and deserialization.
type NotificationHistory struct {
	EventID          types.UUID      `json:"event_id"`
	TriggeredAt      types.UnixMilli `json:"triggered_at"`
	ContactName      types.String    `json:"contact_name"`
	ContactgroupName types.String    `json:"contactgroup_name"`
	ScheduleName     types.String    `json:"schedule_name"`
	ChannelName      string          `json:"channel_name"`
	EventMessage     string          `json:"event_message"`
	IncidentClosed   types.Bool      `json:"incident_closed"`
}
