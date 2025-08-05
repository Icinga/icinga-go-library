package types

import (
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"github.com/pkg/errors"
	"strconv"
)

var (
	enum = map[bool]string{
		true:  "y",
		false: "n",
	}
)

// Bool represents a bool for ENUM ('y', 'n'), which can be NULL.
type Bool struct {
	Bool  bool
	Valid bool // Valid is true if Bool is not NULL
}

// TransformZeroBoolToNull is a transformer function that sets the Valid field to false if the Bool is zero.
// This is useful when you want to convert a zero value to a NULL value in a database context.
func TransformZeroBoolToNull(b *Bool) {
	if b.Valid && !b.Bool {
		b.Valid = false
	}
}

// MakeBool constructs a new Bool.
//
// Multiple transformer functions can be given, each transforming the generated Bool to whatever is needed.
// If no transformers are given, the Bool will be valid and set to the given value.
func MakeBool(bi bool, transformers ...func(*Bool)) Bool {
	b := Bool{Bool: bi, Valid: true}

	for _, transformer := range transformers {
		transformer(&b)
	}

	return b
}

// IsZero implements the json.isZeroer interface.
// A Bool is considered zero if its Valid field is false regardless of its actual Bool value.
func (b Bool) IsZero() bool { return !b.Valid }

// MarshalJSON implements the json.Marshaler interface.
func (b Bool) MarshalJSON() ([]byte, error) {
	if !b.Valid {
		return []byte("null"), nil
	}

	return MarshalJSON(b.Bool)
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (b *Bool) UnmarshalText(text []byte) error {
	parsed, err := strconv.ParseUint(string(text), 10, 64)
	if err != nil {
		return CantParseUint64(err, string(text))
	}

	*b = Bool{parsed != 0, true}
	return nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (b *Bool) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || len(data) == 0 {
		return nil
	}

	if err := UnmarshalJSON(data, &b.Bool); err != nil {
		return err
	}

	b.Valid = true

	return nil
}

// Scan implements the sql.Scanner interface.
// Supports SQL NULL.
func (b *Bool) Scan(src interface{}) error {
	if src == nil {
		b.Bool, b.Valid = false, false
		return nil
	}

	v, ok := src.([]byte)
	if !ok {
		return errors.Errorf("bad []byte type assertion from %#v", src)
	}

	switch string(v) {
	case "y":
		b.Bool = true
	case "n":
		b.Bool = false
	default:
		return errors.Errorf("bad bool %#v", v)
	}

	b.Valid = true

	return nil
}

// Value implements the driver.Valuer interface.
// Supports SQL NULL.
func (b Bool) Value() (driver.Value, error) {
	if !b.Valid {
		return nil, nil
	}

	return enum[b.Bool], nil
}

// Assert interface compliance.
var (
	_ json.Marshaler           = Bool{}
	_ encoding.TextUnmarshaler = (*Bool)(nil)
	_ json.Unmarshaler         = (*Bool)(nil)
	_ sql.Scanner              = (*Bool)(nil)
	_ driver.Valuer            = Bool{}
)
