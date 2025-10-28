package event

import (
	"testing"

	"database/sql/driver"
	"github.com/icinga/icinga-go-library/testutils"
)

func TestType(t *testing.T) {
	t.Parallel()

	t.Run("MarshalJSON", func(t *testing.T) {
		t.Parallel()

		testdata := []testutils.TestCase[string, Type]{
			{Name: "Unknown", Expected: "null", Data: TypeUnknown, Error: nil},
			{Name: "State", Expected: `"state"`, Data: TypeState, Error: nil},
			{Name: "Mute", Expected: `"mute"`, Data: TypeMute, Error: nil},
			{Name: "Unmute", Expected: `"unmute"`, Data: TypeUnmute, Error: nil},
			{Name: "DowntimeStart", Expected: `"downtime-start"`, Data: TypeDowntimeStart, Error: nil},
		}

		for _, tt := range testdata {
			t.Run(tt.Name, tt.F(func(typ Type) (string, error) {
				data, err := typ.MarshalJSON()
				return string(data), err
			}))
		}
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		t.Parallel()

		testData := []testutils.TestCase[Type, string]{
			{Name: "Unknown", Expected: TypeUnknown, Data: "null", Error: nil},
			{Name: "State", Expected: TypeState, Data: `"state"`, Error: nil},
			{Name: "Mute", Expected: TypeMute, Data: `"mute"`, Error: nil},
			{Name: "Unmute", Expected: TypeUnmute, Data: `"unmute"`, Error: nil},
			{Name: "DowntimeStart", Expected: TypeDowntimeStart, Data: `"downtime-start"`, Error: nil},
			{Name: "Invalid", Expected: TypeUnknown, Data: `"invalid"`, Error: testutils.ErrorContains(`unknown type "invalid"`)},
			{Name: "Invalid Unknown", Expected: TypeUnknown, Data: `"unknown"`, Error: testutils.ErrorContains(`unknown type "unknown"`)},
		}

		for _, tt := range testData {
			t.Run(tt.Name, tt.F(func(input string) (Type, error) {
				var tType Type
				return tType, tType.UnmarshalJSON([]byte(input))
			}))
		}
	})

	t.Run("Scan", func(t *testing.T) {
		t.Parallel()

		testdata := []testutils.TestCase[Type, any]{
			{Name: "Unknown", Expected: TypeUnknown, Data: nil, Error: nil},
			{Name: "State", Expected: TypeState, Data: `state`, Error: nil},
			{Name: "Mute", Expected: TypeMute, Data: `mute`, Error: nil},
			{Name: "Unmute", Expected: TypeUnmute, Data: `unmute`, Error: nil},
			{Name: "DowntimeStart", Expected: TypeDowntimeStart, Data: `downtime-start`, Error: nil},
			{Name: "Invalid", Expected: TypeUnknown, Data: `invalid`, Error: testutils.ErrorContains(`unknown type "invalid"`)},
		}

		for _, tt := range testdata {
			t.Run(tt.Name, tt.F(func(input any) (Type, error) {
				var tType Type
				return tType, tType.Scan(input)
			}))
		}
	})

	t.Run("Value", func(t *testing.T) {
		t.Parallel()

		testdata := []testutils.TestCase[driver.Value, Type]{
			{Name: "Unknown", Expected: nil, Data: TypeUnknown, Error: nil},
			{Name: "State", Expected: `state`, Data: TypeState, Error: nil},
			{Name: "Mute", Expected: `mute`, Data: TypeMute, Error: nil},
			{Name: "Unmute", Expected: `unmute`, Data: TypeUnmute, Error: nil},
			{Name: "DowntimeStart", Expected: `downtime-start`, Data: TypeDowntimeStart, Error: nil},
		}

		for _, tt := range testdata {
			t.Run(tt.Name, tt.F(func(typ Type) (driver.Value, error) { return typ.Value() }))
		}
	})
}
