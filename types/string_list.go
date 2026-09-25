package types

import (
	"bytes"
	"database/sql/driver"
	"fmt"
	"time"
)

type NullList[T int64 | float64 | bool | []byte | string | time.Time] struct {
	Elements []T
	Valid    bool // Valid is true if Elements is not NULL
}

func (a *NullList[T]) Scan(src any) error {
	if src == nil {
		a.Elements, a.Valid = nil, false
		return nil
	}

	switch v := src.(type) {
	case []T:
		a.Elements, a.Valid = v, true
		return nil
	default:
		return fmt.Errorf("NullList: unsupported type %T", src)
	}
}

func (a NullList[T]) Value() (driver.Value, error) {
	if !a.Valid {
		return nil, nil
	}
	return a.Elements, nil
}

type StringList struct {
	NullList[string]
}

// TransformEmptyStringListToNull transforms a valid StringList carrying an empty slice to a SQL NULL.
func TransformEmptyStringListToNull(l *StringList) {
	if l.Valid && len(l.Elements) == 0 {
		l.Valid = false
	}
}

// TransformNilStringListToNull transforms a valid StringList carrying a nil slice to a SQL NULL.
func TransformNilStringListToNull(l *StringList) {
	if l.Valid && l.Elements == nil {
		l.Valid = false
	}
}

// MakeStringList constructs a new StringList.
//
// Multiple transformer functions can be given, each transforming the generated StringList, e.g., TransformEmptyStringListToNull.
func MakeStringList(in []string, transformers ...func(*StringList)) StringList {
	l := StringList{Elements: in, Valid: true}

	for _, transformer := range transformers {
		transformer(&l)
	}

	return l
}

func (l StringList) IsZero() bool { return !l.Valid }

// MarshalJSON implements json.Marshaler.
func (l StringList) MarshalJSON() ([]byte, error) {
	var v any
	if l.Valid {
		v = l.Elements
	}

	return MarshalJSON(v)
}

// UnmarshalJSON implements json.Unmarshaler.
func (l *StringList) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package.
	if bytes.HasPrefix(data, []byte{'n'}) {
		return nil
	}

	if string(data) == "null" {
		l.Elements, l.Valid = nil, false
		return nil
	}

	if err := UnmarshalJSON(data, &l.Elements); err != nil {
		return err
	}

	l.Valid = true

	return nil
}

//// UnmarshalText implements encoding.TextUnmarshaler.
//func (l *StringList) UnmarshalText(data []byte) error {
//	if len(data) == 0 {
//		l.Elements, l.Valid = nil, false
//	}
//	t := string(data)
//	//truncate the square brackets
//	t = strings.Trim(t, "[")
//	t = strings.Trim(t, "]")
//	tS := strings.Split(t, ",")
//	for k, v := range tS {
//		tS[k] = strings.Trim(v, `"`)
//	}
//
//	l.Elements = tS
//	l.Valid = true
//
//	return nil
//}
