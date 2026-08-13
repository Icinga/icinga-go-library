package types

import (
	"bytes"
	"database/sql/driver"
	"encoding"
	"encoding/binary"

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

// Assert interface compliance.
var (
	_ encoding.TextUnmarshaler = (*UUID)(nil)
	_ driver.Valuer            = UUID{}
)

func UUIDFromBinaries(a, b []byte) UUID {
	buf := make([]byte, 0, 8+len(a)+8+len(b))

	if bytes.Compare(a, b) > 0 {
		a, b = b, a
	}

	var lenA [8]byte
	binary.BigEndian.PutUint64(lenA[:], uint64(len(a)))
	buf = append(buf, lenA[:]...)
	buf = append(buf, a...)

	var lenB [8]byte
	binary.BigEndian.PutUint64(lenB[:], uint64(len(b)))
	buf = append(buf, lenB[:]...)
	buf = append(buf, b...)

	return UUID{UUID: uuid.NewSHA1(uuid.Nil, buf)}
}
