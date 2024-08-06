package logging

import (
	"github.com/icinga/icinga-go-library/config"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"testing"
	"time"
)

func TestConfig(t *testing.T) {
	subtests := []struct {
		name     string
		opts     config.EnvOptions
		expected Config
		error    bool
	}{
		{
			name: "empty",
			opts: config.EnvOptions{},
			expected: Config{
				Output:   "console",
				Interval: 20 * time.Second,
			},
		},
		{
			name:  "invalid-output",
			opts:  config.EnvOptions{Environment: map[string]string{"OUTPUT": "☃"}},
			error: true,
		},
		{
			name: "customized",
			opts: config.EnvOptions{Environment: map[string]string{
				"LEVEL":    zapcore.DebugLevel.String(),
				"OUTPUT":   JOURNAL,
				"INTERVAL": "3m14s",
			}},
			expected: Config{
				Level:    zapcore.DebugLevel,
				Output:   JOURNAL,
				Interval: 3*time.Minute + 14*time.Second,
			},
		},
		{
			name: "options",
			opts: config.EnvOptions{Environment: map[string]string{"OPTIONS": "foo:debug,bar:info,buz:panic"}},
			expected: Config{
				Output:   "console",
				Interval: 20 * time.Second,
				Options: map[string]zapcore.Level{
					"foo": zapcore.DebugLevel,
					"bar": zapcore.InfoLevel,
					"buz": zapcore.PanicLevel,
				},
			},
		},
		{
			name: "options-as-ints",
			opts: config.EnvOptions{Environment: map[string]string{"OPTIONS": "foo:-1,bar:0,buz:4"}},
			expected: Config{
				Output:   "console",
				Interval: 20 * time.Second,
				Options: map[string]zapcore.Level{
					"foo": zapcore.DebugLevel,
					"bar": zapcore.InfoLevel,
					"buz": zapcore.PanicLevel,
				},
			},
		},
	}

	for _, test := range subtests {
		t.Run(test.name, func(t *testing.T) {
			var out Config
			if err := config.FromEnv(&out, test.opts); test.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expected, out)
			}
		})
	}
}
