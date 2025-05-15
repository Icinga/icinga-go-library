package logging

import (
	"fmt"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/pkg/errors"
	"github.com/ssgreg/journald"
	"go.uber.org/zap/zapcore"
	"strings"
)

// journaldPriorities maps zapcore.Level to journal.Priority.
var journaldPriorities = map[zapcore.Level]journald.Priority{
	zapcore.DebugLevel:  journald.PriorityDebug,
	zapcore.InfoLevel:   journald.PriorityInfo,
	zapcore.WarnLevel:   journald.PriorityWarning,
	zapcore.ErrorLevel:  journald.PriorityErr,
	zapcore.FatalLevel:  journald.PriorityCrit,
	zapcore.PanicLevel:  journald.PriorityCrit,
	zapcore.DPanicLevel: journald.PriorityCrit,
}

// journaldVisibleFields is a set (map to struct{}) of field keys being logged within the message for journald.
var journaldVisibleFields = map[string]struct{}{
	"error": {},
}

// NewJournaldCore returns a zapcore.Core that sends log entries to systemd-journald and
// uses the given identifier as a prefix for structured logging context that is sent as journal fields.
func NewJournaldCore(identifier string, enab zapcore.LevelEnabler) zapcore.Core {
	return &journaldCore{
		LevelEnabler: enab,
		identifier:   identifier,
	}
}

type journaldCore struct {
	zapcore.LevelEnabler
	context    []zapcore.Field
	identifier string
}

func (c *journaldCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}

	return ce
}

func (c *journaldCore) Sync() error {
	return nil
}

func (c *journaldCore) With(fields []zapcore.Field) zapcore.Core {
	cc := *c
	cc.context = append(cc.context[:len(cc.context):len(cc.context)], fields...)

	return &cc
}

func (c *journaldCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	pri, ok := journaldPriorities[ent.Level]
	if !ok {
		return errors.Errorf("unknown log level %q", ent.Level)
	}

	enc := zapcore.NewMapObjectEncoder()
	c.addFields(enc, fields)
	c.addFields(enc, c.context)
	enc.Fields["SYSLOG_IDENTIFIER"] = c.identifier

	// Re-encode keys before passing them to journald. Unfortunately, this cannot be done within addFields or at another
	// earlier position since zapcore's Field.AddTo may create multiple entries, some with non-compliant names.
	encFields := make(map[string]interface{})
	for k, v := range enc.Fields {
		encFields[encodeJournaldFieldKey(k)] = v
	}

	message := ent.Message + visibleFieldsMsg(journaldVisibleFields, append(fields, c.context...))
	if ent.LoggerName != c.identifier {
		message = ent.LoggerName + ": " + message
	}

	return journald.Send(message, pri, encFields)
}

// addFields adds all given fields to enc with an altered key, prefixed with the journaldCore.identifier.
func (c *journaldCore) addFields(enc zapcore.ObjectEncoder, fields []zapcore.Field) {
	for _, field := range fields {
		field.Key = c.identifier + "_" + field.Key
		field.AddTo(enc)
	}
}

// encodeJournaldFieldKey alters a string to be used as a journald field key.
//
// When journald receives a field with an invalid key, it silently discards this field. This makes syntactically correct
// keys a necessity. Unfortunately, there was no specific documentation about the field key syntax available. This
// function follows the logic enforced in systemd's journal_field_valid function[0].
//
// This boils down to:
// - Key length MUST be within (0, 64] characters.
// - Key MUST start with [A-Z].
// - Key characters MUST be [A-Z0-9_].
//
//	[0]: https://github.com/systemd/systemd/blob/11d5e2b5fbf9f6bfa5763fd45b56829ad4f0777f/src/libsystemd/sd-journal/journal-file.c#L1703
func encodeJournaldFieldKey(key string) string {
	if len(key) == 0 {
		// While this is definitely an error, panicking would be too destructive and silently dropping fields is against
		// the very idea of ensuring key conformity.
		return "EMPTY_KEY"
	}

	isAsciiUpper := func(r rune) bool { return 'A' <= r && r <= 'Z' }
	isAsciiDigit := func(r rune) bool { return '0' <= r && r <= '9' }

	keyParts := []rune(strcase.ScreamingSnake(key))
	for i, r := range keyParts {
		if isAsciiUpper(r) || isAsciiDigit(r) || r == '_' {
			continue
		}
		keyParts[i] = '_'
	}
	key = string(keyParts)

	if !isAsciiUpper(rune(key[0])) {
		// Escape invalid leading characters with a generic "ESC_" prefix. This was seen as a safer choice instead of
		// iterating over the key and removing parts.
		key = "ESC_" + key
	}

	if len(key) > 64 {
		key = key[:64]
	}

	return key
}

// visibleFieldsMsg creates a string to be appended to the log message including fields to be explicitly printed.
//
// When logging against journald, the zapcore.Fields are used as journald fields, resulting in not being shown in the
// default journalctl output (short). While this is documented in our docs, missing error messages are usually confusing
// for end users.
//
// This method takes an allow list (set, map of keys to empty struct) of key to be displayed - there is the global
// variable journaldVisibleFields; parameter for testing - and a slice of zapcore.Fields, creating an output string of
// the allowed fields prefixed by a whitespace separator. If there are no fields to be logged, the returned string is
// empty. So the function output can be appended to the output message without further checks.
func visibleFieldsMsg(visibleFieldKeys map[string]struct{}, fields []zapcore.Field) string {
	if visibleFieldKeys == nil || fields == nil {
		return ""
	}

	enc := zapcore.NewMapObjectEncoder()

	for _, field := range fields {
		if _, shouldLog := visibleFieldKeys[field.Key]; shouldLog {
			field.AddTo(enc)
		}
	}

	// The internal zapcore.encodeError function[^0] can result in multiple fields. For example, an error type
	// implementing fmt.Formatter results in another "errorVerbose" field, containing the stack trace if the error was
	// created by github.com/pkg/errors including a stack[^1]. So the keys are checked again in the following loop.
	//
	// [^0]: https://github.com/uber-go/zap/blob/v1.27.0/zapcore/error.go#L47
	// [^1]: https://pkg.go.dev/github.com/pkg/errors@v0.9.1#WithStack
	visibleFields := make([]string, 0, len(visibleFieldKeys))
	for k, v := range utils.IterateOrderedMap(enc.Fields) {
		if _, shouldLog := visibleFieldKeys[k]; !shouldLog {
			continue
		}

		var encodedField string
		switch v.(type) {
		case string, []byte, error:
			encodedField = fmt.Sprintf("%s=%q", k, v)
		default:
			encodedField = fmt.Sprintf(`%s="%v"`, k, v)
		}

		visibleFields = append(visibleFields, encodedField)
	}

	if len(visibleFields) == 0 {
		return ""
	}

	return "\t" + strings.Join(visibleFields, ", ")
}
