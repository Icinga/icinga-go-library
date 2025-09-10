//go:generate go tool stringer -linecomment -type Severity -output severity_string.go

package event

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Severity represents the severity level of an event in Icinga notifications.
// It is an integer type with predefined constants for different severity levels.
type Severity uint8

const (
	SeverityNone Severity = iota // none

	SeverityOK      // ok
	SeverityDebug   // debug
	SeverityInfo    // info
	SeverityNotice  // notice
	SeverityWarning // warning
	SeverityErr     // err
	SeverityCrit    // crit
	SeverityAlert   // alert
	SeverityEmerg   // emerg

	severityMax // internal
)

// MarshalJSON implements the [json.Marshaler] interface for Severity.
func (s Severity) MarshalJSON() ([]byte, error) {
	if s != SeverityNone {
		return json.Marshal(s.String())
	} else {
		return json.Marshal(nil)
	}
}

// UnmarshalJSON implements the [json.Unmarshaler] interface for Severity.
func (s *Severity) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = SeverityNone
		return nil
	}

	var severityStr string
	if err := json.Unmarshal(data, &severityStr); err != nil {
		return err
	}

	severity, err := ParseSeverity(severityStr)
	if err != nil {
		return err
	}

	*s = severity
	return nil
}

// Scan implements the [sql.Scanner] interface for Severity.
// Supports SQL NULL values.
func (s *Severity) Scan(src any) error {
	if src == nil {
		*s = SeverityNone
		return nil
	}

	var severityStr string
	switch val := src.(type) {
	case string:
		severityStr = val
	case []byte:
		severityStr = string(val)
	default:
		return fmt.Errorf("cannot scan severity from type %T", src)
	}

	severity, err := ParseSeverity(severityStr)
	if err != nil {
		return err
	}

	*s = severity
	return nil
}

// Value implements the [driver.Valuer] interface for Severity.
func (s Severity) Value() (driver.Value, error) {
	if s != SeverityNone {
		return s.String(), nil
	}
	return nil, nil // Return nil for SeverityNone or invalid values
}

// ParseSeverity parses a string representation of a severity level and returns the corresponding Severity value.
// If the string does not match any known severity, it returns an error indicating the unknown severity.
func ParseSeverity(name string) (Severity, error) {
	for s := range severityMax {
		if s.String() == name {
			return s, nil
		}
	}

	return SeverityNone, fmt.Errorf("unknown severity %q", name)
}
