package database

import (
	"context"
	"database/sql/driver"
	"fmt"
	"github.com/creasty/defaults"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDatabaseUtils(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := GetTestDB(ctx, t, "ICINGAGOLIBRARY")

	t.Run("SetMySQLSessionVars", func(t *testing.T) {
		t.Parallel()
		if db.DriverName() != MySQL {
			t.Skipf("skipping set session vars test for %q driver", db.DriverName())
		}

		setMysqlSessionVars(ctx, db, t)
	})

	t.Run("InsertObtainID", func(t *testing.T) {
		t.Parallel()

		defer func() {
			_, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS igl_test_insert_obtain")
			assert.NoError(t, err, "dropping test database table should not fail")
		}()

		var err error
		if db.DriverName() == PostgreSQL {
			_, err = db.ExecContext(ctx, "CREATE TABLE igl_test_insert_obtain (id SERIAL PRIMARY KEY, name VARCHAR(255))")
		} else {
			_, err = db.ExecContext(ctx, "CREATE TABLE igl_test_insert_obtain (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255))")
		}
		require.NoError(t, err, "creating test database table should not fail")

		id, err := InsertObtainID(ctx, db, "INSERT INTO igl_test_insert_obtain (name) VALUES (:name)", map[string]any{"name": "test1"})
		require.NoError(t, err, "inserting new row into test database table should not fail")
		assert.Equal(t, id, int64(1))

		id, err = InsertObtainID(ctx, db, "INSERT INTO igl_test_insert_obtain (name) VALUES (:name)", map[string]any{"name": "test2"})
		require.NoError(t, err, "inserting new row into test database table should not fail")
		assert.Equal(t, id, int64(2))
	})
}

func setMysqlSessionVars(ctx context.Context, db *DB, t *testing.T) {
	vars := map[string][]struct {
		name   string
		value  string
		expect error
	}{
		"UnknownVariables": {
			// MySQL single nodes do not recognise the "wsrep_sync_wait" system variable, but MariaDB does!
			{name: "wsrep_sync_wait", value: "15"}, // MySQL unknown sys var | MariaDB succeeds
			{name: "wsrep_sync_wait", value: "7"},  // MySQL unknown sys var | MariaDB succeeds
			// Just some random unknown system variables :-)
			{name: "Icinga", value: "Icinga"},     // unknown sys var
			{name: "IcingaDB", value: "IcingaDB"}, // unknown sys var
		},
		"VariablesWithCorrectValue": { // Setting system variables known by MySQL/MariaDB to a valid value
			{name: "autocommit", value: "true"},
			{name: "binlog_format", value: "MIXED"},
			{name: "completion_type", value: "1" /** CHAIN */},
			{name: "completion_type", value: "CHAIN"},
			{name: "default_storage_engine", value: "InnoDB"},
		},
		"VariablesWithInvalidValues": { // System variables set to an invalid value
			{name: "autocommit", value: "SOMETHING", expect: &mysql.MySQLError{Number: 1231}},
			{name: "binlog_format", value: "IcingaDB", expect: &mysql.MySQLError{Number: 1231}},          // Invalid val!
			{name: "completion_type", value: "-10", expect: &mysql.MySQLError{Number: 1231}},             // Min valid val 0
			{name: "default_storage_engine", value: "IcingaDB", expect: &mysql.MySQLError{Number: 1286}}, // Unknown storage Engine!
		},
	}

	for name, vs := range vars {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			for _, v := range vs {
				conn, err := db.Conn(ctx)
				require.NoError(t, err, "connecting to MySQL/MariaDB database should not fail")

				err = conn.Raw(func(conn any) error {
					c, ok := conn.(driver.Conn)
					if !ok {
						return fmt.Errorf("conn is not a driver.Conn")
					}
					return unsafeSetSessionVariableIfExists(ctx, c, v.name, v.value)
				})

				assert.ErrorIsf(t, err, v.expect, "setting %q variable to '%v' returns unexpected result", v.name, v.value)
				assert.NoError(t, conn.Close(), "closing MySQL/MariaDB connection should not fail")
			}
		})
	}
}

// GetTestDB retrieves the database config from env variables, opens a new database and returns it.
// The [envPrefix] argument defines the environment variables prefix to look for e.g. `ICINGAGOLIBRARY`.
//
// The test suite will be skipped if no `envPrefix+"_TESTS_DB_TYPE" environment variable is
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
