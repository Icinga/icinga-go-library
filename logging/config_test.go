package logging

import (
	"fmt"
	"github.com/creasty/defaults"
	"github.com/icinga/icinga-go-library/config"
	"github.com/icinga/icinga-go-library/testutils"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"os"
	"testing"
	"time"
)

func TestConfig(t *testing.T) {
	var defaultConfig Config
	require.NoError(t, defaults.Set(&defaultConfig), "setting default config")

	configTests := []testutils.TestCase[Config, testutils.ConfigTestData]{
		{
			Name: "Defaults",
			Data: testutils.ConfigTestData{
				// An empty YAML file causes an error,
				// so specify a valid key without a value to trigger fallback to the default.
				Yaml: `level:`,
			},
			Expected: defaultConfig,
		},
		{
			Name: "periodic logging interval must be positive",
			Data: testutils.ConfigTestData{
				Yaml: `interval: 0s`,
				Env:  map[string]string{"INTERVAL": "0s"},
			},
			Error: testutils.ErrorContains("periodic logging interval must be positive"),
		},
		{
			Name: "invalid logger output",
			Data: testutils.ConfigTestData{
				Yaml: `output: invalid`,
				Env:  map[string]string{"OUTPUT": "invalid"},
			},
			Error: testutils.ErrorContains("invalid is not a valid logger output"),
		},
		{
			Name: "Customized",
			Data: testutils.ConfigTestData{
				Yaml: fmt.Sprintf(
					`
level: debug
output: %s
interval: 3m14s`,
					JOURNAL,
				),
				Env: map[string]string{
					"LEVEL":    zapcore.DebugLevel.String(),
					"OUTPUT":   JOURNAL,
					"INTERVAL": "3m14s",
				},
			},
			Expected: Config{
				Level:    zapcore.DebugLevel,
				Output:   JOURNAL,
				Interval: 3*time.Minute + 14*time.Second,
			},
		},
		{
			Name: "Options",
			Data: testutils.ConfigTestData{
				Yaml: `
options:
  foo: debug
  bar: info
  buz: panic`,
				Env: map[string]string{"OPTIONS": "foo:debug,bar:info,buz:panic"},
			},
			Expected: Config{
				Output:   defaultConfig.Output,
				Interval: defaultConfig.Interval,
				Options: map[string]zapcore.Level{
					"foo": zapcore.DebugLevel,
					"bar": zapcore.InfoLevel,
					"buz": zapcore.PanicLevel,
				},
			},
		},
		{
			Name: "Options with invalid level",
			Data: testutils.ConfigTestData{
				Yaml: `
options:
  foo: foo`,
				Env: map[string]string{"OPTIONS": "foo:foo"},
			},
			Error: testutils.ErrorContains(`unrecognized level: "foo"`),
		},
	}

	t.Run("FromEnv", func(t *testing.T) {
		for _, tc := range configTests {
			t.Run(tc.Name, tc.F(func(data testutils.ConfigTestData) (Config, error) {
				var actual Config

				err := config.FromEnv(&actual, config.EnvOptions{Environment: data.Env})

				return actual, err
			}))
		}
	})

	t.Run("FromYAMLFile", func(t *testing.T) {
		for _, tc := range configTests {
			t.Run(tc.Name+"/FromYAMLFile", tc.F(func(data testutils.ConfigTestData) (Config, error) {
				var actual Config

				var err error
				testutils.WithYAMLFile(t, data.Yaml, func(file *os.File) {
					err = config.FromYAMLFile(file.Name(), &actual)
				})

				return actual, err
			}))
		}
	})
}
