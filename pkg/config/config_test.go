package config

import (
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestFromYAMLFile(t *testing.T) {
	type Config struct {
		Key string `yaml:"key"`
		mockValidatorValid
	}

	type testCase struct {
		name     string
		content  string
		expected *Config
	}

	tests := []testCase{
		{
			name:    "Valid YAML",
			content: `key: value`,
			expected: &Config{
				Key: "value",
			},
		},
		{
			name:    "Empty YAML",
			content: "",
		},
		{
			name:    "Empty YAML with directive separator",
			content: "---",
		},
		{
			name:    "Invalid YAML",
			content: ":\n",
		},
		//{
		//	name: "Non-Existent File",
		//	setup: func() (string, func()) {
		//		return "nonexistent.yaml", func() {}
		//	},
		//	expectError: true,
		//},
		//{
		//	name: "Permission Denied",
		//	setup: func() (string, func()) {
		//		tmpFile, _ := os.CreateTemp("", "*.yaml")
		//		tmpFile.Chmod(0000)
		//		tmpFile.Close()
		//		return tmpFile.Name(), func() { os.Remove(tmpFile.Name()) }
		//	},
		//	expectError: true,
		//},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			yamlFile, err := os.CreateTemp("", "*.yaml")
			require.NoError(t, err)
			defer func(name string) {
				_ = os.Remove(name)
			}(yamlFile.Name())
			err = os.WriteFile(yamlFile.Name(), []byte(tc.content), 0600)
			require.NoError(t, err)

			config, err := FromYAMLFile[Config](yamlFile.Name())
			if tc.expected == nil {
				require.Error(t, err)
			} else {
				require.Equal(t, tc.expected, config)
			}
		})
	}
}

func TestParseFlags(t *testing.T) {
	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
	}()

	os.Args = []string{"cmd", "--test-flag=value"}

	type Flags struct {
		TestFlag string `long:"test-flag"`
	}

	flags, err := ParseFlags[Flags]()
	require.NoError(t, err)
	require.Equal(t, "value", flags.TestFlag)
}

type mockValidatorValid struct{}

func (m *mockValidatorValid) Validate() error {
	return nil
}

var invalidConfiguration = errors.New("invalid configuration")

type mockValidatorInvalid struct{}

func (m *mockValidatorInvalid) Validate() error {
	return invalidConfiguration
}
