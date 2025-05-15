package logging

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

// testingStackError is an error mimicking the stack behavior from github.com/pkg/errors in a deterministic way.
type testingStackError string

func (err testingStackError) Error() string {
	return string(err)
}

func (err testingStackError) Format(s fmt.State, verb rune) {
	if verb == 'v' && s.Flag('+') {
		_, _ = fmt.Fprintf(s, "%s: look, I am a stack trace", string(err))
	} else {
		_, _ = fmt.Fprintf(s, "%s", string(err))
	}
}

func Test_visibleFieldsMsg(t *testing.T) {
	tests := []struct {
		name             string
		visibleFieldKeys map[string]struct{}
		fields           []zapcore.Field
		output           string
	}{
		{
			name:             "empty-all-nil",
			visibleFieldKeys: nil,
			fields:           nil,
			output:           "",
		},
		{
			name:             "empty-all",
			visibleFieldKeys: map[string]struct{}{},
			fields:           nil,
			output:           "",
		},
		{
			name:             "empty-visibleFiledKeys",
			visibleFieldKeys: map[string]struct{}{},
			fields:           []zapcore.Field{zap.String("foo", "bar")},
			output:           "",
		},
		{
			name:             "no-field-match",
			visibleFieldKeys: map[string]struct{}{"bar": {}},
			fields:           []zapcore.Field{zap.String("foo", "bar")},
			output:           "",
		},
		{
			name:             "expected-string",
			visibleFieldKeys: map[string]struct{}{"foo": {}},
			fields:           []zapcore.Field{zap.String("foo", "bar")},
			output:           "\t" + `foo="bar"`,
		},
		{
			name:             "expected-multiple-strings-with-excluded",
			visibleFieldKeys: map[string]struct{}{"foo": {}, "bar": {}},
			fields: []zapcore.Field{
				zap.String("foo", "bar"),
				zap.String("bar", "baz"),
				zap.String("baz", "qux"), // not in allow list
			},
			output: "\t" + `bar="baz", foo="bar"`,
		},
		{
			name:             "expected-error-simple",
			visibleFieldKeys: map[string]struct{}{"error": {}},
			fields:           []zapcore.Field{zap.Error(fmt.Errorf("oops"))},
			output:           "\t" + `error="oops"`,
		},
		{
			name:             "expected-error-without-stack",
			visibleFieldKeys: map[string]struct{}{"error": {}},
			fields:           []zapcore.Field{zap.Error(errors.WithStack(fmt.Errorf("oops")))},
			output:           "\t" + `error="oops"`,
		},
		{
			name:             "expected-error-with-stack",
			visibleFieldKeys: map[string]struct{}{"error": {}, "errorVerbose": {}},
			fields:           []zapcore.Field{zap.Error(testingStackError("oops"))},
			output:           "\t" + `error="oops", errorVerbose="oops: look, I am a stack trace"`,
		},
		{
			name: "expected-multiple-basic-types",
			visibleFieldKeys: map[string]struct{}{
				"bool":        {},
				"byte-string": {},
				"complex":     {},
				"float":       {},
				"int":         {},
			},
			fields: []zapcore.Field{
				zap.Bool("bool", true),
				zap.ByteString("byte-string", []byte{0xC0, 0xFF, 0xEE}),
				zap.Complex64("complex", -1i),
				zap.Float64("float", 1.0/3.0),
				zap.Int("int", 42),
			},
			output: "\t" + `bool="true", byte-string="\xc0\xff\xee", complex="(0-1i)", float="0.3333333333333333", int="42"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := visibleFieldsMsg(test.visibleFieldKeys, test.fields)
			require.Equal(t, test.output, out)
		})
	}
}
