package config

import (
	"encoding/json"
	"fmt"
	"github.com/icinga/icinga-go-library/testutils"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"io/fs"
	"os"
	"os/exec"
	"reflect"
	"testing"
)

// errInvalidConfiguration is an error that indicates invalid configuration.
var errInvalidConfiguration = errors.New("invalid configuration")

// validateValid is a struct used to represent a valid configuration.
type validateValid struct{}

// Validate returns nil indicating the configuration is valid.
func (_ *validateValid) Validate() error {
	return nil
}

// validateInvalid is a struct used to represent an invalid configuration.
type validateInvalid struct{}

// Validate returns errInvalidConfiguration indicating the configuration is invalid.
func (_ *validateInvalid) Validate() error {
	return errInvalidConfiguration
}

// simpleConfig is an always valid test configuration struct with only one key.
type simpleConfig struct {
	Key string `yaml:"key" env:"KEY"`
	validateValid
}

// inlinedConfigPart is a part of a test configuration that will be inlined.
type inlinedConfigPart struct {
	Key string `yaml:"inlined-key" env:"INLINED_KEY"`
}

// inlinedConfig is an always valid test configuration struct with a key and an inlined part from inlinedConfigPart.
type inlinedConfig struct {
	Key     string            `yaml:"key" env:"KEY"`
	Inlined inlinedConfigPart `yaml:",inline"`
	validateValid
}

// embeddedConfigPart is a part of a test configuration that will be embedded.
type embeddedConfigPart struct {
	Key string `yaml:"embedded-key" env:"EMBEDDED_KEY"`
}

// embeddedConfig is an always valid test configuration struct with a key and an embedded part from embeddedConfigPart.
type embeddedConfig struct {
	Key      string             `yaml:"key" env:"KEY"`
	Embedded embeddedConfigPart `yaml:"embedded" envPrefix:"EMBEDDED_"`
	validateValid
}

// defaultConfigPart is a part of a test configuration that defines a default value.
type defaultConfigPart struct {
	Key string `yaml:"default-key" env:"DEFAULT_KEY" default:"default-value"`
}

// defaultConfig is an always valid test configuration struct with a key and
// an inlined part with defaults from defaultConfigPart.
type defaultConfig struct {
	Key     string            `yaml:"key"  env:"KEY"`
	Default defaultConfigPart `yaml:",inline"`
	validateValid
}

// invalidConfig is an always invalid test configuration struct with only one key.
type invalidConfig struct {
	Key string `yaml:"key" env:"KEY"`
	validateInvalid
}

// configWithInvalidDefault is a test configuration struct used to verify error propagation from defaults.Set().
// It intentionally defines an invalid default value for a map,
// which the defaults package parses using json.Unmarshal().
// The test then asserts that a json.SyntaxError is returned.
// This approach is necessary because the defaults package does not return errors for parsing scalar types,
// which was quite unexpected when writing the test.
type configWithInvalidDefault struct {
	Key                string      `yaml:"key" env:"KEY"`
	InvalidDefaultJson map[any]any `yaml:"invalid" envPrefix:"INVALID_" default:"a"`
	validateValid
}

// nonStructValidator is a non-struct type that implements the Validator interface but
// cannot be used in FromEnv and FromYAMLFile to parse configuration into.
type nonStructValidator int

func (nonStructValidator) Validate() error {
	return nil
}

// configTests specifies common test cases for the FromEnv and FromYAMLFile functions.
var configTests = []testutils.TestCase[Validator, testutils.ConfigTestData]{
	{
		Name: "Simple Config",
		Data: testutils.ConfigTestData{
			Yaml: `key: value`,
			Env:  map[string]string{"KEY": "value"},
		},
		Expected: &simpleConfig{
			Key: "value",
		},
	},
	{
		Name: "Inlined Config",
		Data: testutils.ConfigTestData{
			Yaml: `
key: value
inlined-key: inlined-value`,
			Env: map[string]string{
				"KEY":         "value",
				"INLINED_KEY": "inlined-value",
			}},
		Expected: &inlinedConfig{
			Key:     "value",
			Inlined: inlinedConfigPart{Key: "inlined-value"},
		},
	},
	{
		Name: "Embedded Config",
		Data: testutils.ConfigTestData{
			Yaml: `
key: value
embedded:
  embedded-key: embedded-value`,
			Env: map[string]string{
				"KEY":                   "value",
				"EMBEDDED_EMBEDDED_KEY": "embedded-value",
			}},
		Expected: &embeddedConfig{
			Key:      "value",
			Embedded: embeddedConfigPart{Key: "embedded-value"},
		},
	},
	{
		Name: "Defaults",
		Data: testutils.ConfigTestData{
			Yaml: `key: value`,
			Env:  map[string]string{"KEY": "value"}},
		Expected: &defaultConfig{
			Key:     "value",
			Default: defaultConfigPart{Key: "default-value"},
		},
	},
	{
		Name: "Overriding Defaults",
		Data: testutils.ConfigTestData{
			Yaml: `
key: value
default-key: overridden-value`,
			Env: map[string]string{
				"KEY":         "value",
				"DEFAULT_KEY": "overridden-value",
			}},
		Expected: &defaultConfig{
			Key:     "value",
			Default: defaultConfigPart{Key: "overridden-value"},
		},
	},
	{
		Name: "Validate invalid",
		Data: testutils.ConfigTestData{
			Yaml: `key: value`,
			Env:  map[string]string{"KEY": "value"},
		},
		Expected: &invalidConfig{
			Key: "value",
		},
		Error: testutils.ErrorIs(errInvalidConfiguration),
	},
	{
		Name: "Error propagation from defaults.Set()",
		Data: testutils.ConfigTestData{
			Yaml: `key: value`,
			Env:  map[string]string{"KEY": "value"},
		},
		Expected: &configWithInvalidDefault{},
		Error:    testutils.ErrorAs[*json.SyntaxError](),
	},
}

func TestFromEnv(t *testing.T) {
	for _, tc := range configTests {
		t.Run(tc.Name, tc.F(func(data testutils.ConfigTestData) (Validator, error) {
			// Since our test cases only define the expected configuration,
			// we need to create a new instance of that type for FromEnv to parse the configuration into.
			actual := reflect.New(reflect.TypeOf(tc.Expected).Elem()).Interface().(Validator)

			err := FromEnv(actual, EnvOptions{Environment: data.Env})

			return actual, err
		}))
	}

	t.Run("Nil pointer argument", func(t *testing.T) {
		var config *struct{ Validator }

		err := FromEnv(config, EnvOptions{})
		require.ErrorIs(t, err, ErrInvalidArgument)
	})

	t.Run("Nil argument", func(t *testing.T) {
		err := FromEnv(nil, EnvOptions{})
		require.ErrorIs(t, err, ErrInvalidArgument)
	})

	t.Run("Non-struct pointer argument", func(t *testing.T) {
		var config nonStructValidator

		err := FromEnv(&config, EnvOptions{})
		// Struct pointer assertion is done in the defaults library,
		// so we must ensure that the error returned is not one of our own errors.
		require.NotErrorIs(t, err, ErrInvalidArgument)
		require.NotErrorIs(t, err, errInvalidConfiguration)
	})
}

func TestFromYAMLFile(t *testing.T) {
	for _, tc := range configTests {
		t.Run(tc.Name, tc.F(func(data testutils.ConfigTestData) (Validator, error) {
			// Since our test cases only define the expected configuration,
			// we need to create a new instance of that type for FromYAMLFile to parse the configuration into.
			actual := reflect.New(reflect.TypeOf(tc.Expected).Elem()).Interface().(Validator)

			var err error
			testutils.WithYAMLFile(t, data.Yaml, func(file *os.File) {
				err = FromYAMLFile(file.Name(), actual)
			})

			return actual, err
		}))
	}

	type invalidYamlTestCase struct {
		// Test case name.
		name string
		// Content of the YAML file.
		content string
	}

	invalidYamlTests := []invalidYamlTestCase{
		{
			name:    "Empty YAML",
			content: "",
		},
		{
			name:    "Empty YAML with directive separator",
			content: `---`,
		},
		{
			name:    "Faulty YAML",
			content: `:\n`,
		},
		{
			name:    "Key only",
			content: `key`,
		},
	}

	for _, tc := range invalidYamlTests {
		t.Run(tc.name, func(t *testing.T) {
			testutils.WithYAMLFile(t, tc.content, func(file *os.File) {
				err := FromYAMLFile(file.Name(), &validateValid{})
				require.Error(t, err)
				// Since the YAML library does not export all possible error types,
				// we must ensure that the error returned is not one of our own errors.
				require.NotErrorIs(t, err, ErrInvalidArgument)
				require.NotErrorIs(t, err, errInvalidConfiguration)
			})
		})
	}

	t.Run("Nil pointer argument", func(t *testing.T) {
		var config *struct{ Validator }

		err := FromYAMLFile("", config)
		require.ErrorIs(t, err, ErrInvalidArgument)
	})

	t.Run("Nil argument", func(t *testing.T) {
		err := FromYAMLFile("", nil)
		require.ErrorIs(t, err, ErrInvalidArgument)
	})

	t.Run("Non-existent file", func(t *testing.T) {
		var config struct{ validateValid }
		var pathError *fs.PathError

		err := FromYAMLFile("nonexistent.yaml", &config)
		require.ErrorAs(t, err, &pathError)
		require.ErrorIs(t, pathError.Err, fs.ErrNotExist)
	})

	t.Run("Permission denied", func(t *testing.T) {
		var config struct{ validateValid }
		var pathError *fs.PathError

		yamlFile, err := os.CreateTemp("", "*.yaml")
		require.NoError(t, err)
		require.NoError(t, yamlFile.Chmod(0000))
		require.NoError(t, yamlFile.Close())
		defer func(name string) {
			_ = os.Remove(name)
		}(yamlFile.Name())

		err = FromYAMLFile(yamlFile.Name(), &config)
		require.ErrorAs(t, err, &pathError)
	})
}

func TestParseFlags(t *testing.T) {
	t.Run("Simple flags", func(t *testing.T) {
		originalArgs := os.Args
		defer func() {
			os.Args = originalArgs
		}()

		os.Args = []string{"cmd", "--test-flag=value"}

		type Flags struct {
			TestFlag string `long:"test-flag"`
		}

		var flags Flags
		err := ParseFlags(&flags)
		require.NoError(t, err)
		require.Equal(t, "value", flags.TestFlag)
	})

	t.Run("Nil pointer argument", func(t *testing.T) {
		var flags *any

		err := ParseFlags(flags)
		require.ErrorIs(t, err, ErrInvalidArgument)
	})

	t.Run("Nil argument", func(t *testing.T) {
		err := ParseFlags(nil)
		require.ErrorIs(t, err, ErrInvalidArgument)
	})

	t.Run("Exit on help flag", func(t *testing.T) {
		// This test case checks the behavior of ParseFlags() when the help flag (e.g. -h) is provided.
		// Since ParseFlags() calls os.Exit() upon encountering the help flag, we need to run this
		// test in a separate subprocess to capture and verify the output without terminating the
		// main test process.
		if os.Getenv("TEST_HELP_FLAG") == "1" {
			// This block runs in the subprocess.
			type Flags struct{}
			var flags Flags

			originalArgs := os.Args
			defer func() {
				os.Args = originalArgs
			}()

			os.Args = []string{"cmd", "-h"}

			if err := ParseFlags(&flags); err != nil {
				panic(err)
			}

			return
		}

		// This block runs in the main test process. It starts this test again in a subprocess with the
		// TEST_HELP_FLAG=1 environment variable provided in order to run the above code block.
		// #nosec G204 -- The subprocess is launched with controlled input for testing purposes.
		// The command and arguments are derived from the test framework and are not influenced by external input.
		cmd := exec.Command(os.Args[0], fmt.Sprintf("-test.run=%s", t.Name()))
		cmd.Env = append(os.Environ(), "TEST_HELP_FLAG=1")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err)
		// When the help flag is provided, ParseFlags() outputs usage information,
		// including "-h, --help Show this help message" (whitespace may vary).
		require.Contains(t, string(out), "-h, --help")
	})
}
