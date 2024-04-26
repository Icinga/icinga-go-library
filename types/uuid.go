package types

import (
	"database/sql/driver"
	"encoding"
	"github.com/google/uuid"
)

// UUID is like uuid.UUID, but marshals itself binarily (not like xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx) in SQL context.
type UUID struct {
	uuid.UUID
}

// Value implements driver.Valuer.
func (uuid UUID) Value() (driver.Value, error) {
	return uuid.UUID[:], nil
}

// Scan implements sql.Scanner.
//func (u *UUID) Scan(src interface{}) error {
//	switch v := src.(type) {
//	case []byte:
//		// Assuming src is a UUID in byte slice form, copy it into the UUID field.
//		u, err := uuid.FromBytes(v)
//		if err != nil {
//			return fmt.Errorf("uuid.Scan: %w", err)
//		}
//		uuid.UUID = u
//		return nil
//	case string:
//		// If src is a string, attempt to parse it as a UUID.
//		u, err := uuid.UUID.Parse(v)
//		if err != nil {
//			return fmt.Errorf("uuid.Scan: %w", err)
//		}
//		uuid.UUID = u
//		return nil
//	case nil:
//		// If src is nil, reset the UUID to its zero value.
//		uuid.UUID = uuid.UUID{}
//		return nil
//	default:
//		return fmt.Errorf("uuid.Scan: cannot scan type %T into UUID", src)
//	}
//}

// Assert interface compliance.
var (
	_ encoding.TextUnmarshaler = (*UUID)(nil)
	_ driver.Valuer            = UUID{}
	_ driver.Valuer            = (*UUID)(nil)
	// _ sql.Scanner              = (*UUID)(nil) // Ensure UUID implements sql.Scanner
)
