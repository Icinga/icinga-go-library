package event

import (
	"database/sql/driver"
	"testing"

	"github.com/icinga/icinga-go-library/testutils"
)

func TestSeverity(t *testing.T) {
	t.Parallel()

	t.Run("MarshalJson", func(t *testing.T) {
		t.Parallel()

		testdata := []testutils.TestCase[string, Severity]{
			{Name: "None", Expected: "null", Data: SeverityNone, Error: nil},
			{Name: "Ok", Expected: `"ok"`, Data: SeverityOK, Error: nil},
			{Name: "Warning", Expected: `"warning"`, Data: SeverityWarning, Error: nil},
			{Name: "Error", Expected: `"error"`, Data: SeverityError, Error: nil},
			{Name: "Critical", Expected: `"critical"`, Data: SeverityCritical, Error: nil},
		}

		for _, tt := range testdata {
			t.Run(tt.Name, tt.F(func(s Severity) (string, error) {
				data, err := s.MarshalJSON()
				return string(data), err
			}))
		}
	})

	t.Run("UnmarshalJson", func(t *testing.T) {
		t.Parallel()

		testData := []testutils.TestCase[Severity, string]{
			{Name: "None", Expected: SeverityNone, Data: `null`, Error: nil},
			{Name: "Ok", Expected: SeverityOK, Data: `"ok"`, Error: nil},
			{Name: "Warning", Expected: SeverityWarning, Data: `"warning"`, Error: nil},
			{Name: "Error", Expected: SeverityError, Data: `"error"`, Error: nil},
			{Name: "Critical", Expected: SeverityCritical, Data: `"critical"`, Error: nil},
			{Name: "Invalid", Expected: SeverityNone, Data: `"invalid"`, Error: testutils.ErrorContains(`unknown severity "invalid"`)},
			{Name: "Invalid None", Expected: SeverityNone, Data: `"none"`, Error: testutils.ErrorContains(`unknown severity "none"`)},
		}

		for _, tt := range testData {
			t.Run(tt.Name, tt.F(func(input string) (Severity, error) {
				var s Severity
				return s, s.UnmarshalJSON([]byte(input))
			}))
		}
	})

	t.Run("Scan", func(t *testing.T) {
		t.Parallel()

		testData := []testutils.TestCase[Severity, any]{
			{Name: "None", Expected: SeverityNone, Data: nil, Error: nil},
			{Name: "Ok", Expected: SeverityOK, Data: `ok`, Error: nil},
			{Name: "Warning", Expected: SeverityWarning, Data: `warning`, Error: nil},
			{Name: "Error", Expected: SeverityError, Data: `error`, Error: nil},
			{Name: "Critical", Expected: SeverityCritical, Data: `critical`, Error: nil},
			{Name: "Critical Bytes", Expected: SeverityCritical, Data: []byte("critical"), Error: nil},
			{Name: "Invalid Number", Expected: SeverityNone, Data: 150, Error: testutils.ErrorContains(`cannot scan severity from type int`)},
			{Name: "Invalid String", Expected: SeverityNone, Data: `invalid`, Error: testutils.ErrorContains(`unknown severity "invalid"`)},
		}

		for _, tt := range testData {
			t.Run(tt.Name, tt.F(func(input any) (Severity, error) {
				var s Severity
				return s, s.Scan(input)
			}))
		}
	})

	t.Run("Value", func(t *testing.T) {
		t.Parallel()

		testdata := []testutils.TestCase[driver.Value, Severity]{
			{Name: "None", Expected: nil, Data: SeverityNone, Error: nil},
			{Name: "Ok", Expected: `ok`, Data: SeverityOK, Error: nil},
			{Name: "Warning", Expected: `warning`, Data: SeverityWarning, Error: nil},
			{Name: "Error", Expected: `error`, Data: SeverityError, Error: nil},
			{Name: "Critical", Expected: `critical`, Data: SeverityCritical, Error: nil},
		}

		for _, tt := range testdata {
			t.Run(tt.Name, tt.F(func(s Severity) (driver.Value, error) { return s.Value() }))
		}
	})
}
