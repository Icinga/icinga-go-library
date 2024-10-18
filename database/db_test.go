package database

import (
	"github.com/icinga/icinga-go-library/config"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"testing"
)

func TestNewDbFromConfig_GetAddr(t *testing.T) {
	tests := []struct {
		name string
		conf *Config
		addr string
	}{
		{
			name: "mysql-simple",
			conf: &Config{
				Type:     "mysql",
				Host:     "example.com",
				Database: "db",
				User:     "user",
			},
			addr: "mysql://user@example.com:3306/db",
		},
		{
			name: "mysql-custom-port",
			conf: &Config{
				Type:     "mysql",
				Host:     "example.com",
				Port:     1234,
				Database: "db",
				User:     "user",
			},
			addr: "mysql://user@example.com:1234/db",
		},
		{
			name: "mysql-tls",
			conf: &Config{
				Type:       "mysql",
				Host:       "example.com",
				Database:   "db",
				User:       "user",
				TlsOptions: config.TLS{Enable: true},
			},
			addr: "mysql+tls://user@example.com:3306/db",
		},
		{
			name: "mysql-unix-domain-socket",
			conf: &Config{
				Type:     "mysql",
				Host:     "/var/empty/mysql.sock",
				Database: "db",
				User:     "user",
			},
			addr: "mysql://user@(/var/empty/mysql.sock)/db",
		},
		{
			name: "pgsql-simple",
			conf: &Config{
				Type:     "pgsql",
				Host:     "example.com",
				Database: "db",
				User:     "user",
			},
			addr: "pgsql://user@example.com:5432/db",
		},
		{
			name: "pgsql-custom-port",
			conf: &Config{
				Type:     "pgsql",
				Host:     "example.com",
				Port:     1234,
				Database: "db",
				User:     "user",
			},
			addr: "pgsql://user@example.com:1234/db",
		},
		{
			name: "pgsql-tls",
			conf: &Config{
				Type:       "pgsql",
				Host:       "example.com",
				Database:   "db",
				User:       "user",
				TlsOptions: config.TLS{Enable: true},
			},
			addr: "pgsql+tls://user@example.com:5432/db",
		},
		{
			name: "pgsql-unix-domain-socket",
			conf: &Config{
				Type:     "pgsql",
				Host:     "/var/empty/pgsql",
				Database: "db",
				User:     "user",
			},
			addr: "pgsql://user@(/var/empty/pgsql/.s.PGSQL.5432)/db",
		},
		{
			name: "pgsql-unix-domain-socket-custom-port",
			conf: &Config{
				Type:     "pgsql",
				Host:     "/var/empty/pgsql",
				Port:     1234,
				Database: "db",
				User:     "user",
			},
			addr: "pgsql://user@(/var/empty/pgsql/.s.PGSQL.1234)/db",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, err := NewDbFromConfig(
				test.conf,
				logging.NewLogger(zaptest.NewLogger(t).Sugar(), 0),
				RetryConnectorCallbacks{})
			require.NoError(t, err)
			require.Equal(t, test.addr, db.GetAddr())
		})
	}
}
