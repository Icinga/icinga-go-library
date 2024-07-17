package config

import (
	"encoding/json"
	"fmt"
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
	Key string `yaml:"key"`
	validateValid
}

// inlinedConfigPart is a part of a test configuration that will be inlined.
type inlinedConfigPart struct {
	Key string `yaml:"inlined-key"`
}

// inlinedConfig is an always valid test configuration struct with a key and an inlined part from inlinedConfigPart.
type inlinedConfig struct {
	Key     string            `yaml:"key"`
	Inlined inlinedConfigPart `yaml:",inline"`
	validateValid
}

// embeddedConfigPart is a part of a test configuration that will be embedded.
type embeddedConfigPart struct {
	Key string `yaml:"embedded-key"`
}

// embeddedConfig is an always valid test configuration struct with a key and an embedded part from embeddedConfigPart.
type embeddedConfig struct {
	Key      string             `yaml:"key"`
	Embedded embeddedConfigPart `yaml:"embedded"`
	validateValid
}

// defaultConfigPart is a part of a test configuration that defines a default value.
type defaultConfigPart struct {
	Key string `yaml:"default-key" default:"default-value"`
}

// defaultConfig is an always valid test configuration struct with a key and
// an inlined part with defaults from defaultConfigPart.
type defaultConfig struct {
	Key     string            `yaml:"key"`
	Default defaultConfigPart `yaml:",inline"`
	validateValid
}

// invalidConfig is an always invalid test configuration struct with only one key.
type invalidConfig struct {
	Key string `yaml:"key"`
	validateInvalid
}

// configWithInvalidDefault is a test configuration struct used to verify error propagation from defaults.Set().
// It intentionally defines an invalid default value for a map,
// which the defaults package parses using json.Unmarshal().
// The test then asserts that a json.SyntaxError is returned.
// This approach is necessary because the defaults package does not return errors for parsing scalar types,
// which was quite unexpected when writing the test.
type configWithInvalidDefault struct {
	Key                string      `yaml:"key"`
	InvalidDefaultJson map[any]any `yaml:"valid" default:"a"`
	validateValid
}

func TestFromYAMLFile(t *testing.T) {
	type yamlTestCase struct {
		// Test case name.
		name string
		// Content of the YAML file.
		content string
		// Expected configuration. Empty if parsing the content is expected to produce an error.
		expected Validator
		// Indicates if the configuration is expected to be invalid (by returning errInvalidConfiguration).
		invalid bool
	}

	yamlTests := []yamlTestCase{
		{
			name:    "Simple YAML",
			content: `key: value`,
			expected: &simpleConfig{
				Key: "value",
			},
		},
		{
			name: "Inlined YAML",
			content: `
key: value
inlined-key: inlined-value`,
			expected: &inlinedConfig{
				Key:     "value",
				Inlined: inlinedConfigPart{Key: "inlined-value"},
			},
		},
		{
			name: "Embedded YAML",
			content: `
key: value
embedded:
  embedded-key: embedded-value`,
			expected: &embeddedConfig{
				Key:      "value",
				Embedded: embeddedConfigPart{Key: "embedded-value"},
			},
		},
		{
			name:    "Defaults",
			content: `key: value`,
			expected: &defaultConfig{
				Key:     "value",
				Default: defaultConfigPart{Key: "default-value"},
			},
		},
		{
			name: "Overriding Defaults",
			content: `
key: value
default-key: overridden-value`,
			expected: &defaultConfig{
				Key:     "value",
				Default: defaultConfigPart{Key: "overridden-value"},
			},
		},
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
			name:    "Invalid YAML",
			content: `key: value`,
			expected: &invalidConfig{
				Key: "value",
			},
			invalid: true,
		},
	}

	for _, tc := range yamlTests {
		t.Run(tc.name, func(t *testing.T) {
			yamlFile, err := os.CreateTemp("", "*.yaml")
			require.NoError(t, err)

			defer func(name string) {
				_ = os.Remove(name)
			}(yamlFile.Name())

			require.NoError(t, os.WriteFile(yamlFile.Name(), []byte(tc.content), 0600))

			if tc.expected != nil {
				// Since our test cases only define the expected configuration,
				// we need to create a new instance of that type for FromYAMLFile to parse the configuration into.
				actual := reflect.New(reflect.TypeOf(tc.expected).Elem()).Interface().(Validator)
				err := FromYAMLFile(yamlFile.Name(), actual)
				if !tc.invalid {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, errInvalidConfiguration)
				}

				require.Equal(t, tc.expected, actual)
			} else {
				err := FromYAMLFile(yamlFile.Name(), &validateValid{})
				require.Error(t, err)
				// Assert that error is a parsing error.
				require.NotErrorIs(t, err, ErrInvalidArgument)
				require.NotErrorIs(t, err, errInvalidConfiguration)
			}
		})
	}

	t.Run("Error propagation from defaults.Set()", func(t *testing.T) {
		var config configWithInvalidDefault
		var syntaxError *json.SyntaxError

		yamlFile, err := os.CreateTemp("", "*.yaml")
		require.NoError(t, err)
		require.NoError(t, yamlFile.Close())
		defer func(name string) {
			_ = os.Remove(name)
		}(yamlFile.Name())

		require.NoError(t, os.WriteFile(yamlFile.Name(), []byte(`key: value`), 0600))

		err = FromYAMLFile(yamlFile.Name(), &config)
		require.ErrorAs(t, err, &syntaxError)
	})

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

type simpleValidator struct {
	Foo int `env:"FOO"`
}

func (sv simpleValidator) Validate() error {
	if sv.Foo == 42 {
		return nil
	} else {
		return errors.New("invalid value")
	}
}

type nonStructValidator int

func (nonStructValidator) Validate() error {
	return nil
}

type defaultValidator struct {
	Foo int `env:"FOO" default:"42"`
}

func (defaultValidator) Validate() error {
	return nil
}

type prefixValidator struct {
	Nested simpleValidator `envPrefix:"PREFIX_"`
}

func (prefixValidator) Validate() error {
	return nil
}

func TestFromEnv(t *testing.T) {
	subtests := []struct {
		name    string
		env     map[string]string
		options EnvOptions
		io      Validator
		error   bool
	}{
		{"nil", nil, EnvOptions{}, nil, true},
		{"nonptr", nil, EnvOptions{}, simpleValidator{}, true},
		{"nilptr", nil, EnvOptions{}, (*simpleValidator)(nil), true},
		{"defaulterr", nil, EnvOptions{}, new(nonStructValidator), true},
		{"parseeerr", map[string]string{"FOO": "bar"}, EnvOptions{}, &simpleValidator{}, true},
		{"invalid", map[string]string{"FOO": "23"}, EnvOptions{}, &simpleValidator{}, true},
		{"simple", map[string]string{"FOO": "42"}, EnvOptions{}, &simpleValidator{42}, false},
		{"default", nil, EnvOptions{}, &defaultValidator{42}, false},
		{"override", map[string]string{"FOO": "23"}, EnvOptions{}, &defaultValidator{23}, false},
		{"prefix", map[string]string{"PREFIX_FOO": "42"}, EnvOptions{Prefix: "PREFIX_"}, &simpleValidator{42}, false},
		{"nested", map[string]string{"PREFIX_FOO": "42"}, EnvOptions{}, &prefixValidator{simpleValidator{42}}, false},
	}

	allEnv := make(map[string]struct{})
	for _, st := range subtests {
		for k := range st.env {
			allEnv[k] = struct{}{}
		}
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			for k := range allEnv {
				require.NoError(t, os.Unsetenv(k))
			}

			for k, v := range st.env {
				require.NoError(t, os.Setenv(k, v))
			}

			var actual Validator
			if vActual := reflect.ValueOf(st.io); vActual != (reflect.Value{}) {
				if vActual.Kind() == reflect.Ptr && !vActual.IsNil() {
					vActual = reflect.New(vActual.Type().Elem())
				}

				actual = vActual.Interface().(Validator)
			}

			if err := FromEnv(actual, st.options); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.io, actual)
			}
		})
	}
}
