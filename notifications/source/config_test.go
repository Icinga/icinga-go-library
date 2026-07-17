package source

import (
	"fmt"
	"os"
	"testing"

	"github.com/icinga/icinga-go-library/config"
	"github.com/icinga/icinga-go-library/testutils"
	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	passwordFile, cleanupPasswordFile := testutils.PasswordFile(t, "secret")
	defer cleanupPasswordFile()

	configTests := []testutils.TestCase[Config, testutils.ConfigTestData]{
		{
			Name: "HTTP config",
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
			Name: "HTTP config with password file",
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
		{
			Name: "HTTP config missing credentials",
			Data: testutils.ConfigTestData{
				Yaml: `url: http://localhost:5680`,
				Env: map[string]string{
					"URL": "http://localhost:5680",
				},
			},
			Error: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "requires a username and password")
			},
		},
		{
			Name: "HTTPS config",
			Data: testutils.ConfigTestData{
				Yaml: `
url: https://localhost:5680
username: icinga
password: secret`,
				Env: map[string]string{
					"URL":      "https://localhost:5680",
					"USERNAME": "icinga",
					"PASSWORD": "secret",
				},
			},
			Expected: Config{
				Url:        "https://localhost:5680",
				Username:   "icinga",
				Password:   "secret",
				TlsOptions: config.TLS{TLSCommon: config.TLSCommon{Enable: true}},
			},
		},
		{
			Name: "HTTPS config with password file",
			Data: testutils.ConfigTestData{
				Yaml: fmt.Sprintf(`
url: https://localhost:5680
username: icinga
password_file: %s`, passwordFile),
				Env: map[string]string{
					"URL":           "https://localhost:5680",
					"USERNAME":      "icinga",
					"PASSWORD_FILE": passwordFile,
				},
			},
			Expected: Config{
				Url:          "https://localhost:5680",
				Username:     "icinga",
				Password:     "secret",
				PasswordFile: passwordFile,
				TlsOptions:   config.TLS{TLSCommon: config.TLSCommon{Enable: true}},
			},
		},
		{
			Name: "HTTPS config with client cert",
			Data: testutils.ConfigTestData{
				Yaml: `
url: https://localhost:5680
cert: /client.crt
key: /client.key`,
				Env: map[string]string{
					"URL":  "https://localhost:5680",
					"CERT": "/client.crt",
					"KEY":  "/client.key",
				},
			},
			Expected: Config{
				Url: "https://localhost:5680",
				TlsOptions: config.TLS{TLSCommon: config.TLSCommon{
					Enable: true,
					Cert:   "/client.crt",
					Key:    "/client.key",
				}},
			},
		},
		{
			Name: "HTTPS config missing credentials",
			Data: testutils.ConfigTestData{
				Yaml: `url: https://localhost:5680`,
				Env: map[string]string{
					"URL": "https://localhost:5680",
				},
			},
			Error: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "requires either certificates or username and password")
			},
		},
		{
			Name: "Unix domain socket config",
			Data: testutils.ConfigTestData{
				Yaml: `url: unix:///path/to/socket`,
				Env: map[string]string{
					"URL": "unix:///path/to/socket",
				},
			},
			Expected: Config{
				Url: "unix:///path/to/socket",
			},
		},
		{
			Name: "Unix domain socket config with credentials",
			Data: testutils.ConfigTestData{
				Yaml: `
url: unix:///path/to/socket
username: icinga
password: secret`,
				Env: map[string]string{
					"URL":      "unix:///path/to/socket",
					"USERNAME": "icinga",
					"PASSWORD": "secret",
				},
			},
			Error: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "uses no username/password authentication")
			},
		},
		{
			Name: "Custom attribute negotiation",
			Data: testutils.ConfigTestData{
				Yaml: `
url: http://localhost:5680
username: icinga
password: secret
default_relations:
  - 'host.vars'
  - 'services[*].vars'`,
				Env: map[string]string{
					"URL":               "http://localhost:5680",
					"USERNAME":          "icinga",
					"PASSWORD":          "secret",
					"DEFAULT_RELATIONS": "host.vars,services[*].vars",
				},
			},
			Expected: Config{
				Url:              "http://localhost:5680",
				Username:         "icinga",
				Password:         "secret",
				DefaultRelations: []string{"host.vars", "services[*].vars"},
			},
		},
		{
			Name: "Invalid URL scheme",
			Data: testutils.ConfigTestData{
				Yaml: `url: invalid:nope`,
				Env: map[string]string{
					"URL": "invalid:nope",
				},
			},
			Error: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "unsupported notifications scheme")
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
