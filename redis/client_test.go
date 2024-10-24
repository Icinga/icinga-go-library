package redis

import (
	"github.com/icinga/icinga-go-library/config"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"testing"
)

func TestNewClientFromConfig_GetAddr(t *testing.T) {
	tests := []struct {
		name string
		conf *Config
		addr string
	}{
		{
			name: "redis-simple",
			conf: &Config{
				Host: "example.com",
			},
			addr: "redis://example.com:6379",
		},
		{
			name: "redis-custom-port",
			conf: &Config{
				Host: "example.com",
				Port: 6380,
			},
			addr: "redis://example.com:6380",
		},
		{
			name: "redis-acl",
			conf: &Config{
				Host:     "example.com",
				Username: "user",
				Password: "pass",
			},
			addr: "redis://user@example.com:6379",
		},
		{
			name: "redis-custom-database",
			conf: &Config{
				Host:     "example.com",
				Database: 23,
			},
			addr: "redis://example.com:6379/23",
		},
		{
			name: "redis-tls",
			conf: &Config{
				Host:       "example.com",
				TlsOptions: config.TLS{Enable: true},
			},
			addr: "redis+tls://example.com:6379",
		},
		{
			name: "redis-with-everything",
			conf: &Config{
				Host:       "example.com",
				Port:       6380,
				Username:   "user",
				Password:   "pass",
				Database:   23,
				TlsOptions: config.TLS{Enable: true},
			},
			addr: "redis+tls://user@example.com:6380/23",
		},
		{
			name: "redis-unix-domain-socket",
			conf: &Config{
				Host: "/var/empty/redis.sock",
			},
			addr: "redis://(/var/empty/redis.sock)",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			redis, err := NewClientFromConfig(
				test.conf,
				logging.NewLogger(zaptest.NewLogger(t).Sugar(), 0))
			require.NoError(t, err)
			require.Equal(t, test.addr, redis.GetAddr())
		})
	}
}
