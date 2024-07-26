package logging

import (
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/pkg/errors"
	"github.com/ssgreg/journald"
	"go.uber.org/zap/zapcore"
)

// priorities maps zapcore.Level to journal.Priority.
var priorities = map[zapcore.Level]journald.Priority{
	zapcore.DebugLevel:  journald.PriorityDebug,
	zapcore.InfoLevel:   journald.PriorityInfo,
	zapcore.WarnLevel:   journald.PriorityWarning,
	zapcore.ErrorLevel:  journald.PriorityErr,
	zapcore.FatalLevel:  journald.PriorityCrit,
	zapcore.PanicLevel:  journald.PriorityCrit,
	zapcore.DPanicLevel: journald.PriorityCrit,
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
	pri, ok := priorities[ent.Level]
	if !ok {
		return errors.Errorf("unknown log level %q", ent.Level)
	}

	enc := zapcore.NewMapObjectEncoder()
	// Ensure that all field keys are valid journald field keys. If in doubt, use encodeJournaldFieldKey.
	c.addFields(enc, fields)
	c.addFields(enc, c.context)
	enc.Fields["SYSLOG_IDENTIFIER"] = c.identifier

	message := ent.Message
	if ent.LoggerName != c.identifier {
		message = ent.LoggerName + ": " + message
	}

	return journald.Send(message, pri, enc.Fields)
}

// addFields adds all given fields to enc with an altered key, prefixed with the journaldCore.identifier and sanitized
// via encodeJournaldFieldKey.
func (c *journaldCore) addFields(enc zapcore.ObjectEncoder, fields []zapcore.Field) {
	for _, field := range fields {
		field.Key = encodeJournaldFieldKey(c.identifier + "_" + field.Key)
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
