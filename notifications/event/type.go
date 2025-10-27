//go:generate go tool stringer -linecomment -type Type -output type_string.go

package event

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Type represents the type of event sent to the Icinga Notifications API.
type Type uint8

const (
	TypeUnknown Type = iota // unknown

	TypeAcknowledgementCleared // acknowledgement-cleared
	TypeAcknowledgementSet     // acknowledgement-set
	TypeCustom                 // custom
	TypeDowntimeEnd            // downtime-end
	TypeDowntimeRemoved        // downtime-removed
	TypeDowntimeStart          // downtime-start
	TypeFlappingEnd            // flapping-end
	TypeFlappingStart          // flapping-start
	TypeIncidentAge            // incident-age
	TypeMute                   // mute
	TypeState                  // state
	TypeUnmute                 // unmute

	typeMax // internal
)

// MarshalJSON implements the [json.Marshaler] interface for Type.
func (t Type) MarshalJSON() ([]byte, error) {
	if t != TypeUnknown {
		return json.Marshal(t.String())
	} else {
		return json.Marshal(nil)
	}
}

// UnmarshalJSON implements the [json.Unmarshaler] interface for Type.
func (t *Type) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*t = TypeUnknown
		return nil
	}

	var typeString string
	if err := json.Unmarshal(data, &typeString); err != nil {
		return err
	}

	parsedType, err := ParseType(typeString)
	if err != nil {
		return err
	}

	*t = parsedType
	return nil
}

// Scan implements the [sql.Scanner] interface for Severity.
// Supports SQL NULL values.
func (t *Type) Scan(src any) error {
	if src == nil {
		*t = TypeUnknown
		return nil
	}

	var typeStr string
	switch val := src.(type) {
	case string:
		typeStr = val
	case []byte:
		typeStr = string(val)
	default:
		return fmt.Errorf("cannot scan Type from %T", src)
	}

	parsedType, err := ParseType(typeStr)
	if err != nil {
		return err
	}

	*t = parsedType
	return nil
}

// Value implements the [driver.Valuer] interface for Severity.
func (t Type) Value() (driver.Value, error) {
	if t != TypeUnknown {
		return t.String(), nil
	}
	return nil, nil // Return nil for unknown type
}

// ParseType parses a string into a Type.
//
// If the string does not match any known type, it returns an error indicating the unknown type.
func ParseType(s string) (Type, error) {
	for t := range typeMax {
		if s == t.String() {
			return t, nil
		}
	}

	return TypeUnknown, fmt.Errorf("unknown type %q", s)
}
