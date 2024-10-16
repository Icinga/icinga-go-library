package redis

import (
	"github.com/creasty/defaults"
	"github.com/icinga/icinga-go-library/config"
	"github.com/icinga/icinga-go-library/testutils"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func TestConfig(t *testing.T) {
	var defaultOptions Options
	require.NoError(t, defaults.Set(&defaultOptions), "setting default options")

	configTests := []testutils.TestCase[Config, testutils.ConfigTestData]{
		{
			Name: "Redis host missing",
			Data: testutils.ConfigTestData{
				Yaml: `host:`,
			},
			Error: testutils.ErrorContains("Redis host missing"),
		},
		{
			Name: "Minimal config",
			Data: testutils.ConfigTestData{
				Yaml: `host: localhost`,
				Env:  map[string]string{"HOST": "localhost"},
			},
			Expected: Config{
				Host:    "localhost",
				Options: defaultOptions,
			},
		},
		{
			Name: "Redis password must be set, if username is provided",
			Data: testutils.ConfigTestData{
				Yaml: `
host: localhost
username: username`,
				Env: map[string]string{
					"HOST":     "localhost",
					"USERNAME": "username",
				},
			},
			Error: testutils.ErrorContains("Redis password must be set, if username is provided"),
		},
		{
			Name: "Customized config",
			Data: testutils.ConfigTestData{
				Yaml: `
host: localhost
username: username
password: password
database: 2`,
				Env: map[string]string{
					"HOST":     "localhost",
					"USERNAME": "username",
					"PASSWORD": "password",
					"DATABASE": "2",
				},
			},
			Expected: Config{
				Host:     "localhost",
				Username: "username",
				Password: "password",
				Database: 2,
				Options:  defaultOptions,
			},
		},
		{
			Name: "TLS",
			Data: testutils.ConfigTestData{
				Yaml: `
host: localhost
tls: true
cert: cert.pem
key: key.pem
ca: ca.pem`,
				Env: map[string]string{
					"HOST": "localhost",
					"TLS":  "1",
					"CERT": "cert.pem",
					"KEY":  "key.pem",
					"CA":   "ca.pem",
				},
			},
			Expected: Config{
				Host:    "localhost",
				Options: defaultOptions,
				TlsOptions: config.TLS{
					Enable: true,
					Cert:   "cert.pem",
					Key:    "key.pem",
					Ca:     "ca.pem",
				},
			},
		},
		{
			Name: "block_timeout must be positive",
			Data: testutils.ConfigTestData{
				Yaml: `
host: localhost
options:
  block_timeout: -1s`,
				Env: map[string]string{
					"HOST":                  "localhost",
					"OPTIONS_BLOCK_TIMEOUT": "-1s",
				},
			},
			Error: testutils.ErrorContains("block_timeout must be positive"),
		},
		{
			Name: "hmget_count must be at least 1",
			Data: testutils.ConfigTestData{
				Yaml: `
host: localhost
options:
  hmget_count: 0`,
				Env: map[string]string{
					"HOST":                "localhost",
					"OPTIONS_HMGET_COUNT": "0",
				},
			},
			Error: testutils.ErrorContains("hmget_count must be at least 1"),
		},
		{
			Name: "hscan_count must be at least 1",
			Data: testutils.ConfigTestData{
				Yaml: `
host: localhost
options:
  hscan_count: 0`,
				Env: map[string]string{
					"HOST":                "localhost",
					"OPTIONS_HSCAN_COUNT": "0",
				},
			},
			Error: testutils.ErrorContains("hscan_count must be at least 1"),
		},
		{
			Name: "max_hmget_connections must be at least 1",
			Data: testutils.ConfigTestData{
				Yaml: `
host: localhost
options:
  max_hmget_connections: 0`,
				Env: map[string]string{
					"HOST":                          "localhost",
					"OPTIONS_MAX_HMGET_CONNECTIONS": "0",
				},
			},
			Error: testutils.ErrorContains("max_hmget_connections must be at least 1"),
		},
		{
			Name: "timeout cannot be 0",
			Data: testutils.ConfigTestData{
				Yaml: `
host: localhost
options:
  timeout: 0s`,
				Env: map[string]string{
					"HOST":            "localhost",
					"OPTIONS_TIMEOUT": "0s",
				},
			},
			Error: testutils.ErrorContains("timeout cannot be 0. Configure a value greater than zero, or use -1 for no timeout"),
		},
		{
			Name: "xread_count must be at least 1",
			Data: testutils.ConfigTestData{
				Yaml: `
host: localhost
options:
  xread_count: 0`,
				Env: map[string]string{
					"HOST":                "localhost",
					"OPTIONS_XREAD_COUNT": "0",
				},
			},
			Error: testutils.ErrorContains("xread_count must be at least 1"),
		},
		{
			Name: "Options retain defaults",
			Data: testutils.ConfigTestData{
				Yaml: `
host: localhost
options:
  block_timeout: 2s
  hmget_count: 512`,
				Env: map[string]string{
					"HOST":                  "localhost",
					"OPTIONS_BLOCK_TIMEOUT": "2s",
					"OPTIONS_HMGET_COUNT":   "512",
				},
			},
			Expected: Config{
				Host: "localhost",
				Options: Options{
					BlockTimeout:        2 * time.Second,
					HMGetCount:          512,
					HScanCount:          defaultOptions.HScanCount,
					MaxHMGetConnections: defaultOptions.MaxHMGetConnections,
					Timeout:             defaultOptions.Timeout,
					XReadCount:          defaultOptions.XReadCount,
				},
			},
		},
		{
			Name: "Options",
			Data: testutils.ConfigTestData{
				Yaml: `
host: localhost
options:
  block_timeout: 2s
  hmget_count: 512
  hscan_count: 1024
  max_hmget_connections: 16
  timeout: 60s
  xread_count: 2048`,
				Env: map[string]string{
					"HOST":                          "localhost",
					"OPTIONS_BLOCK_TIMEOUT":         "2s",
					"OPTIONS_HMGET_COUNT":           "512",
					"OPTIONS_HSCAN_COUNT":           "1024",
					"OPTIONS_MAX_HMGET_CONNECTIONS": "16",
					"OPTIONS_TIMEOUT":               "60s",
					"OPTIONS_XREAD_COUNT":           "2048",
				},
			},
			Expected: Config{
				Host: "localhost",
				Options: Options{
					BlockTimeout:        2 * time.Second,
					HMGetCount:          512,
					HScanCount:          1024,
					MaxHMGetConnections: 16,
					Timeout:             60 * time.Second,
					XReadCount:          2048,
				},
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
