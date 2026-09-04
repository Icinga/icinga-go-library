package types

import (
	"database/sql"
	"database/sql/driver"
	"encoding"

	"github.com/google/uuid"
)

// UUID is like uuid.NullUUID, but marshals itself binarily (not like xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx) in SQL.
type UUID struct {
	uuid.NullUUID
}

// TransformNilUUIDToNull transforms a valid UUID carrying a nil value to a SQL NULL.
func TransformNilUUIDToNull(u *UUID) {
	if u.Valid && u.UUID == uuid.Nil {
		u.Valid = false
	}
}

// MakeUUID constructs a new UUID.
//
// Multiple transformer functions can be given, each transforming the generated UUID, e.g., TransformNilUUIDToNull.
func MakeUUID(in uuid.UUID, opts ...func(*UUID)) UUID {
	u := UUID{NullUUID: uuid.NullUUID{UUID: in, Valid: true}}
	for _, opt := range opts {
		opt(&u)
	}
	return u
}

// IsZero implements the json.isZeroer interface.
//
// A NullUUID is considered zero if it is not valid regardless of the actual UUID value.
func (u *UUID) IsZero() bool { return !u.Valid }

// Value implements driver.Valuer.
// Supports SQL NULL.
func (u UUID) Value() (driver.Value, error) {
	if !u.Valid {
		return nil, nil
	}
	return u.UUID[:], nil
}

// String implements fmt.Stringer.
//
// It is necessary to implement this method because the embedded NullUUID's doesn't embed the uuid.UUID type, but has
// it as a field, so the String() method of the UUID field of NullUUID is not promoted.
func (u UUID) String() string {
	if !u.Valid {
		return ""
	}
	return u.UUID.String()
}

// Assert interface compliance.
var (
	_ encoding.TextUnmarshaler = (*UUID)(nil)
	_ driver.Valuer            = UUID{}
	_ sql.Scanner              = (*UUID)(nil)
)
