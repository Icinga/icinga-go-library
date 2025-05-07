package types

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"strings"
)

// String adds JSON support to sql.NullString.
type String struct {
	sql.NullString
}

// TransformEmptyStringToNull transforms a valid String carrying an empty text to a SQL NULL.
func TransformEmptyStringToNull(s *String) {
	if s.Valid && s.String == "" {
		s.Valid = false
	}
}

// MakeString constructs a new String.
//
// Multiple transformer functions can be given, each transforming the generated String, e.g., TransformEmptyStringToNull.
func MakeString(in string, transformers ...func(*String)) String {
	s := String{sql.NullString{
		String: in,
		Valid:  true,
	}}

	for _, transformer := range transformers {
		transformer(&s)
	}

	return s
}

// MarshalJSON implements the json.Marshaler interface.
// Supports JSON null.
func (s String) MarshalJSON() ([]byte, error) {
	var v interface{}
	if s.Valid {
		v = s.String
	}

	return MarshalJSON(v)
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (s *String) UnmarshalText(text []byte) error {
	*s = String{sql.NullString{
		String: string(text),
		Valid:  true,
	}}

	return nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// Supports JSON null.
func (s *String) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package.
	if bytes.HasPrefix(data, []byte{'n'}) {
		return nil
	}

	if err := UnmarshalJSON(data, &s.String); err != nil {
		return err
	}

	s.Valid = true

	return nil
}

// Value implements the driver.Valuer interface.
// Supports SQL NULL.
func (s String) Value() (driver.Value, error) {
	if !s.Valid {
		return nil, nil
	}

	// PostgreSQL does not allow null bytes in varchar, char and text fields.
	return strings.ReplaceAll(s.String, "\x00", ""), nil
}

// Assert interface compliance.
var (
	_ json.Marshaler           = String{}
	_ encoding.TextUnmarshaler = (*String)(nil)
	_ json.Unmarshaler         = (*String)(nil)
	_ driver.Valuer            = String{}
	_ sql.Scanner              = (*String)(nil)
)
