package types

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"strconv"
)

// Int adds JSON support to sql.NullInt64.
type Int struct {
	sql.NullInt64
}

// TransformZeroIntToNull transforms a valid Int carrying a zero value to a SQL NULL.
func TransformZeroIntToNull(i *Int) {
	if i.Valid && i.Int64 == 0 {
		i.Valid = false
	}
}

// MakeInt constructs a new Int.
//
// Multiple transformer functions can be given, each transforming the generated Int, e.g., TransformZeroIntToNull.
func MakeInt(in int64, transformers ...func(*Int)) Int {
	i := Int{sql.NullInt64{
		Int64: in,
		Valid: true,
	}}

	for _, transformer := range transformers {
		transformer(&i)
	}

	return i
}

// MarshalJSON implements the json.Marshaler interface.
// Supports JSON null.
func (i Int) MarshalJSON() ([]byte, error) {
	var v interface{}
	if i.Valid {
		v = i.Int64
	}

	return MarshalJSON(v)
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (i *Int) UnmarshalText(text []byte) error {
	parsed, err := strconv.ParseInt(string(text), 10, 64)
	if err != nil {
		return CantParseInt64(err, string(text))
	}

	*i = Int{sql.NullInt64{
		Int64: parsed,
		Valid: true,
	}}

	return nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// Supports JSON null.
func (i *Int) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package.
	if bytes.HasPrefix(data, []byte{'n'}) {
		return nil
	}

	if err := UnmarshalJSON(data, &i.Int64); err != nil {
		return err
	}

	i.Valid = true

	return nil
}

// Assert interface compliance.
var (
	_ json.Marshaler           = Int{}
	_ json.Unmarshaler         = (*Int)(nil)
	_ encoding.TextUnmarshaler = (*Int)(nil)
	_ driver.Valuer            = Int{}
	_ sql.Scanner              = (*Int)(nil)
)
