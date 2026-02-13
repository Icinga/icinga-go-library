package source

import (
	"fmt"
	"os"
	"testing"

	"github.com/icinga/icinga-go-library/config"
	"github.com/icinga/icinga-go-library/testutils"
)

func TestConfig(t *testing.T) {
	passwordFile, cleanupPasswordFile := testutils.PasswordFile(t, "secret")
	defer cleanupPasswordFile()

	configTests := []testutils.TestCase[Config, testutils.ConfigTestData]{
		{
			Name: "Minimal config",
			Data: testutils.ConfigTestData{
				Yaml: `
url: http://localhost:5680
username: icinga
password: secret`,
				Env: map[string]string{
					"URL":      "http://localhost:5680",
					"USERNAME": "icinga",
					"PASSWORD": "secret",
				},
			},
			Expected: Config{
				Url:      "http://localhost:5680",
				Username: "icinga",
				Password: "secret",
			},
		},
		{
			Name: "Minimal config with password file",
			Data: testutils.ConfigTestData{
				Yaml: fmt.Sprintf(`
url: http://localhost:5680
username: icinga
password_file: %s`, passwordFile),
				Env: map[string]string{
					"URL":           "http://localhost:5680",
					"USERNAME":      "icinga",
					"PASSWORD_FILE": passwordFile,
				},
			},
			Expected: Config{
				Url:          "http://localhost:5680",
				Username:     "icinga",
				Password:     "secret",
				PasswordFile: passwordFile,
			},
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
