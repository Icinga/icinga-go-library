package event

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSeverity(t *testing.T) {
	t.Parallel()

	t.Run("MarshalJson", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			severity Severity
			expected string
		}{
			{SeverityNone, "null"},
			{SeverityOK, `"ok"`},
			{SeverityDebug, `"debug"`},
			{SeverityInfo, `"info"`},
			{SeverityNotice, `"notice"`},
			{SeverityWarning, `"warning"`},
			{SeverityErr, `"err"`},
			{SeverityCrit, `"crit"`},
			{SeverityAlert, `"alert"`},
			{SeverityEmerg, `"emerg"`},
		}

		for _, test := range tests {
			data, err := json.Marshal(test.severity)
			require.NoError(t, err)
			assert.Equal(t, test.expected, string(data))
		}
	})

	t.Run("UnmarshalJson", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			input    string
			expected Severity
			wantErr  bool
		}{
			{`null`, SeverityNone, false},
			{`"ok"`, SeverityOK, false},
			{`"debug"`, SeverityDebug, false},
			{`"info"`, SeverityInfo, false},
			{`"notice"`, SeverityNotice, false},
			{`"warning"`, SeverityWarning, false},
			{`"err"`, SeverityErr, false},
			{`"crit"`, SeverityCrit, false},
			{`"alert"`, SeverityAlert, false},
			{`"emerg"`, SeverityEmerg, false},
			{`"invalid"`, SeverityNone, true}, // Invalid severity
		}

		for _, test := range tests {
			var severity Severity
			err := json.Unmarshal([]byte(test.input), &severity)
			assert.Equalf(t, test.wantErr, err != nil, "expected error: %v, got: %v", test.wantErr, err)
			assert.Equal(t, test.expected, severity)
		}
	})

	t.Run("Scan", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			input    any
			expected Severity
			wantErr  bool
		}{
			{nil, SeverityNone, false},
			{"ok", SeverityOK, false},
			{"debug", SeverityDebug, false},
			{"info", SeverityInfo, false},
			{"notice", SeverityNotice, false},
			{"warning", SeverityWarning, false},
			{"err", SeverityErr, false},
			{"crit", SeverityCrit, false},
			{"alert", SeverityAlert, false},
			{"emerg", SeverityEmerg, false},
			{123, SeverityNone, true},               // Invalid type
			{[]byte("invalid"), SeverityNone, true}, // Invalid severity
		}

		for _, test := range tests {
			var severity Severity
			if err := severity.Scan(test.input); test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, severity)
			}
		}
	})

	t.Run("Value", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			severity Severity
			expected any
		}{
			{SeverityNone, nil},
			{SeverityOK, "ok"},
			{SeverityDebug, "debug"},
			{SeverityInfo, "info"},
			{SeverityNotice, "notice"},
			{SeverityWarning, "warning"},
			{SeverityErr, "err"},
			{SeverityCrit, "crit"},
			{SeverityAlert, "alert"},
			{SeverityEmerg, "emerg"},
		}

		for _, test := range tests {
			value, err := test.severity.Value()
			require.NoError(t, err)
			assert.Equal(t, test.expected, value)
		}
	})
}
