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
			{Name: "Debug", Expected: `"debug"`, Data: SeverityDebug, Error: nil},
			{Name: "Info", Expected: `"info"`, Data: SeverityInfo, Error: nil},
			{Name: "Notice", Expected: `"notice"`, Data: SeverityNotice, Error: nil},
			{Name: "Warning", Expected: `"warning"`, Data: SeverityWarning, Error: nil},
			{Name: "Err", Expected: `"err"`, Data: SeverityErr, Error: nil},
			{Name: "Crit", Expected: `"crit"`, Data: SeverityCrit, Error: nil},
			{Name: "Alert", Expected: `"alert"`, Data: SeverityAlert, Error: nil},
			{Name: "Emerg", Expected: `"emerg"`, Data: SeverityEmerg, Error: nil},
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
			{Name: "Debug", Expected: SeverityDebug, Data: `"debug"`, Error: nil},
			{Name: "Info", Expected: SeverityInfo, Data: `"info"`, Error: nil},
			{Name: "Notice", Expected: SeverityNotice, Data: `"notice"`, Error: nil},
			{Name: "Warning", Expected: SeverityWarning, Data: `"warning"`, Error: nil},
			{Name: "Err", Expected: SeverityErr, Data: `"err"`, Error: nil},
			{Name: "Crit", Expected: SeverityCrit, Data: `"crit"`, Error: nil},
			{Name: "Alert", Expected: SeverityAlert, Data: `"alert"`, Error: nil},
			{Name: "Emerg", Expected: SeverityEmerg, Data: `"emerg"`, Error: nil},
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
			{Name: "Debug", Expected: SeverityDebug, Data: `debug`, Error: nil},
			{Name: "Info", Expected: SeverityInfo, Data: `info`, Error: nil},
			{Name: "Notice", Expected: SeverityNotice, Data: `notice`, Error: nil},
			{Name: "Warning", Expected: SeverityWarning, Data: `warning`, Error: nil},
			{Name: "Err", Expected: SeverityErr, Data: `err`, Error: nil},
			{Name: "Crit", Expected: SeverityCrit, Data: `crit`, Error: nil},
			{Name: "Alert", Expected: SeverityAlert, Data: `alert`, Error: nil},
			{Name: "Alert Bytes", Expected: SeverityAlert, Data: []byte("alert"), Error: nil},
			{Name: "Emerg", Expected: SeverityEmerg, Data: `emerg`, Error: nil},
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
			{Name: "Debug", Expected: `debug`, Data: SeverityDebug, Error: nil},
			{Name: "Info", Expected: `info`, Data: SeverityInfo, Error: nil},
			{Name: "Notice", Expected: `notice`, Data: SeverityNotice, Error: nil},
			{Name: "Warning", Expected: `warning`, Data: SeverityWarning, Error: nil},
			{Name: "Err", Expected: `err`, Data: SeverityErr, Error: nil},
			{Name: "Crit", Expected: `crit`, Data: SeverityCrit, Error: nil},
			{Name: "Alert", Expected: `alert`, Data: SeverityAlert, Error: nil},
			{Name: "Emerg", Expected: `emerg`, Data: SeverityEmerg, Error: nil},
		}

		for _, tt := range testdata {
			t.Run(tt.Name, tt.F(func(s Severity) (driver.Value, error) { return s.Value() }))
		}
	})
}
