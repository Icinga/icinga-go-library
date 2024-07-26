package logging

import (
	"github.com/stretchr/testify/require"
	"regexp"
	"testing"
)

func Test_journaldFieldEncode(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
	}{
		{"empty", "", "EMPTY_KEY"},
		{"lowercase", "foo", "FOO"},
		{"uppercase", "FOO", "FOO"},
		{"dash", "foo-bar", "FOO_BAR"},
		{"non ascii", "snow_☃", "SNOW__"},
		{"lowercase non ascii alpha", "föö", "F__"},
		{"uppercase non ascii alpha", "FÖÖ", "F__"},
		{"leading number", "23", "ESC_23"},
		{"leading underscore", "_foo", "ESC__FOO"},
		{"leading invalid", " foo", "ESC__FOO"},
		{"max length", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA1234", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA1234"},
		{"too long", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA12345", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA1234"},
		{"too long leading number", "1234567890123456789012345678901234567890123456789012345678901234", "ESC_123456789012345678901234567890123456789012345678901234567890"},
		{"concrete example", "icinga-notifications" + "_" + "error", "ICINGA_NOTIFICATIONS_ERROR"},
		{"example syslog_identifier", "SYSLOG_IDENTIFIER", "SYSLOG_IDENTIFIER"},
	}

	check := regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := encodeJournaldFieldKey(test.input)
			require.Equal(t, test.output, out)
			require.True(t, check.MatchString(out), "check regular expression")
		})
	}
}
