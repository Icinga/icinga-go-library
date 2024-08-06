package database

import (
	"github.com/creasty/defaults"
	"github.com/icinga/icinga-go-library/config"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConfig(t *testing.T) {
	var defaultOptions Options
	require.NoError(t, defaults.Set(&defaultOptions), "setting default options")

	subtests := []struct {
		name     string
		opts     config.EnvOptions
		expected Config
		error    bool
	}{
		{
			name:  "empty-missing-fields",
			opts:  config.EnvOptions{},
			error: true,
		},
		{
			name:  "unknown-db-type",
			opts:  config.EnvOptions{Environment: map[string]string{"TYPE": "☃"}},
			error: true,
		},
		{
			name: "minimal-config",
			opts: config.EnvOptions{Environment: map[string]string{
				"HOST":     "db.example.com",
				"USER":     "user",
				"DATABASE": "db",
			}},
			expected: Config{
				Type:     "mysql",
				Host:     "db.example.com",
				Database: "db",
				User:     "user",
				Options:  defaultOptions,
			},
		},
		{
			name: "tls",
			opts: config.EnvOptions{Environment: map[string]string{
				"HOST":     "db.example.com",
				"USER":     "user",
				"DATABASE": "db",
				"TLS":      "true",
				"CERT":     "/var/empty/db.crt",
				"CA":       "/var/empty/ca.crt",
			}},
			expected: Config{
				Type:     "mysql",
				Host:     "db.example.com",
				Database: "db",
				User:     "user",
				TlsOptions: config.TLS{
					Enable: true,
					Cert:   "/var/empty/db.crt",
					Ca:     "/var/empty/ca.crt",
				},
				Options: defaultOptions,
			},
		},
		{
			name: "options",
			opts: config.EnvOptions{Environment: map[string]string{
				"HOST":                             "db.example.com",
				"USER":                             "user",
				"DATABASE":                         "db",
				"OPTIONS_MAX_CONNECTIONS":          "1",
				"OPTIONS_MAX_ROWS_PER_TRANSACTION": "65535",
			}},
			expected: Config{
				Type:     "mysql",
				Host:     "db.example.com",
				Database: "db",
				User:     "user",
				Options: Options{
					MaxConnections:              1,
					MaxConnectionsPerTable:      8,
					MaxPlaceholdersPerStatement: 8192,
					MaxRowsPerTransaction:       65535,
					WsrepSyncWait:               7,
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
