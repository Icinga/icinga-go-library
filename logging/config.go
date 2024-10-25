package logging

import (
	"fmt"
	"github.com/creasty/defaults"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	"os"
	"strings"
	"time"
)

// Options define child loggers with their desired log level.
type Options map[string]zapcore.Level

// UnmarshalText implements encoding.TextUnmarshaler to allow Options to be parsed by env.
//
// This custom TextUnmarshaler is necessary as - for the moment - env does not support map[T]encoding.TextUnmarshaler.
// After <https://github.com/caarlos0/env/pull/323> got merged and a new env release was drafted, this method can be
// removed.
func (o *Options) UnmarshalText(text []byte) error {
	optionsMap := make(map[string]zapcore.Level)

	for _, entry := range strings.Split(string(text), ",") {
		key, valueStr, found := strings.Cut(entry, ":")
		if !found {
			return fmt.Errorf("entry %q cannot be unmarshalled as an Option entry", entry)
		}

		valueLvl, err := zapcore.ParseLevel(valueStr)
		if err != nil {
			return fmt.Errorf("entry %q cannot be unmarshalled as level, %w", entry, err)
		}

		optionsMap[key] = valueLvl
	}

	*o = optionsMap
	return nil
}

// Config defines Logger configuration.
type Config struct {
	// zapcore.Level at 0 is for info level.
	Level  zapcore.Level `yaml:"level" env:"LEVEL" default:"0"`
	Output string        `yaml:"output" env:"OUTPUT"`
	// Interval for periodic logging.
	Interval time.Duration `yaml:"interval" env:"INTERVAL" default:"20s"`

	Options `yaml:"options" env:"OPTIONS"`
}

// SetDefaults implements defaults.Setter to configure the log output if it is not set:
// systemd-journald is used when Icinga DB is running under systemd, otherwise stderr.
func (c *Config) SetDefaults() {
	if defaults.CanUpdate(c.Output) {
		if _, ok := os.LookupEnv("NOTIFY_SOCKET"); ok {
			// When started by systemd, NOTIFY_SOCKET is set by systemd for Type=notify supervised services,
			// which is the default setting for the Icinga DB service.
			// This assumes that Icinga DB is running under systemd, so set output to systemd-journald.
			c.Output = JOURNAL
		} else {
			// Otherwise set it to console, i.e. write log messages to stderr.
			c.Output = CONSOLE
		}
	}
}

// Validate checks constraints in the configuration and returns an error if they are violated.
func (c *Config) Validate() error {
	if c.Interval <= 0 {
		return errors.New("periodic logging interval must be positive")
	}

	return AssertOutput(c.Output)
}

// AssertOutput returns an error if output is not a valid logger output.
func AssertOutput(o string) error {
	if o == CONSOLE || o == JOURNAL {
		return nil
	}

	return invalidOutput(o)
}

func invalidOutput(o string) error {
	return fmt.Errorf("%s is not a valid logger output. Must be either %q or %q", o, CONSOLE, JOURNAL)
}
