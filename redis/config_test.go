package redis

import (
	"github.com/creasty/defaults"
	"github.com/icinga/icinga-go-library/config"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
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
			name:  "empty-missing-host",
			opts:  config.EnvOptions{},
			error: true,
		},
		{
			name: "minimal-config",
			opts: config.EnvOptions{Environment: map[string]string{"HOST": "kv.example.com"}},
			expected: Config{
				Host:    "kv.example.com",
				Options: defaultOptions,
			},
		},
		{
			name: "customized-config",
			opts: config.EnvOptions{Environment: map[string]string{
				"HOST":     "kv.example.com",
				"USERNAME": "user",
				"PASSWORD": "insecure",
				"DATABASE": "23",
			}},
			expected: Config{
				Host:     "kv.example.com",
				Username: "user",
				Password: "insecure",
				Database: 23,
				Options:  defaultOptions,
			},
		},
		{
			name: "tls",
			opts: config.EnvOptions{Environment: map[string]string{
				"HOST": "kv.example.com",
				"TLS":  "true",
				"CERT": "/var/empty/db.crt",
				"CA":   "/var/empty/ca.crt",
			}},
			expected: Config{
				Host: "kv.example.com",
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
				"HOST":                          "kv.example.com",
				"OPTIONS_BLOCK_TIMEOUT":         "1m",
				"OPTIONS_MAX_HMGET_CONNECTIONS": "1000",
			}},
			expected: Config{
				Host: "kv.example.com",
				Options: Options{
					BlockTimeout:        time.Minute,
					HMGetCount:          4096,
					HScanCount:          4096,
					MaxHMGetConnections: 1000,
					Timeout:             30 * time.Second,
					XReadCount:          4096,
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
