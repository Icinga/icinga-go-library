package types

import (
	"database/sql/driver"
	"fmt"
)

// StringList is a nullable slice of strings.
type StringList struct {
	List  []string
	Valid bool
}

// Scan implements the SQL driver.Scanner interface.
func (n *StringList) Scan(src any) error {
	if src == nil {
		n.List, n.Valid = nil, false
		return nil
	}

	switch v := src.(type) {
	case []string:
		n.List, n.Valid = v, true
		return nil
	default:
		return fmt.Errorf("cannot scan type %T into StringList", src)
	}
}

// Value implements the driver Valuer interface.
func (n StringList) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.List, nil
}

// TransformEmptyStringListToNull transforms a valid StringList carrying an empty slice to a SQL NULL.
func TransformEmptyStringListToNull(n *StringList) {
	if n.Valid && len(n.List) == 0 {
		n.Valid = false
	}
}

// MakeStringList constructs a new StringList.
//
// Multiple transformer functions can be given, each transforming the generated StringList, e.g., TransformEmptyStringListToNull.
func MakeStringList(in []string, transformers ...func(*StringList)) StringList {
	n := StringList{List: in, Valid: true}
	for _, transformer := range transformers {
		transformer(&n)
	}
	return n
}

// IsZero implements the json.isZeroer interface.
//
// A StringList is considered zero if it is not valid regardless of the actual List value.
func (n StringList) IsZero() bool { return !n.Valid }

// MarshalJSON implements json.Marshaler.
func (n StringList) MarshalJSON() ([]byte, error) {
	if n.Valid {
		return MarshalJSON(n.List)
	}
	return []byte("null"), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *StringList) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.List, n.Valid = nil, false
		return nil
	}

	if err := UnmarshalJSON(data, &n.List); err != nil {
		return err
	}
	n.Valid = true

	return nil
}
