package database

import (
	"context"
	"github.com/creasty/defaults"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// GetTestDB retrieves the database config from env variables, opens a new database and returns it.
// The [envPrefix] argument defines the environment variables prefix to look for e.g. `NOTIFICATIONS`.
//
// The test suite will be skipped if no `envPrefix+"_NOTIFICATIONS_TESTS_DB_TYPE" environment variable is
// set, otherwise fails fatally when invalid configurations are specified.
func GetTestDB(ctx context.Context, t *testing.T, envPrefix string) *DB {
	c := &Config{}
	require.NoError(t, defaults.Set(c), "applying config default should not fail")

	if v, ok := os.LookupEnv(envPrefix + "_TESTS_DB_TYPE"); ok {
		c.Type = strings.ToLower(v)
	} else {
		t.Skipf("Environment %q not set, skipping test!", envPrefix+"_TESTS_DB_TYPE")
	}

	if v, ok := os.LookupEnv(envPrefix + "_TESTS_DB"); ok {
		c.Database = v
	}
	if v, ok := os.LookupEnv(envPrefix + "_TESTS_DB_USER"); ok {
		c.User = v
	}
	if v, ok := os.LookupEnv(envPrefix + "_TESTS_DB_PASSWORD"); ok {
		c.Password = v
	}
	if v, ok := os.LookupEnv(envPrefix + "_TESTS_DB_HOST"); ok {
		c.Host = v
	}
	if v, ok := os.LookupEnv(envPrefix + "_TESTS_DB_PORT"); ok {
		port, err := strconv.Atoi(v)
		require.NoError(t, err, "invalid port provided")

		c.Port = port
	}

	require.NoError(t, c.Validate(), "database config validation should not fail")

	db, err := NewDbFromConfig(c, logging.NewLogger(zaptest.NewLogger(t).Sugar(), time.Hour), RetryConnectorCallbacks{})
	require.NoError(t, err, "connecting to database should not fail")
	require.NoError(t, db.PingContext(ctx), "pinging the database should not fail")

	return db
}
