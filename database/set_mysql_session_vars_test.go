package database

import (
"context"
"database/sql/driver"
"github.com/go-sql-driver/mysql"
"github.com/jmoiron/sqlx"
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
"testing"
)

func TestSetMysqlSessionVars(t *testing.T) {
	vars := map[string][]struct {
		name   string
		value  any
		expect error
	}{
		"UnknownVariables": {
			// MySQL single nodes do not recognise the "wsrep_sync_wait" system variable, but MariaDB does!
			{name: "wsrep_sync_wait", value: int64(15)}, // MySQL unknown sys var | MariaDB succeeds
			{name: "wsrep_sync_wait", value: int64(7)},  // MySQL unknown sys var | MariaDB succeeds
			// Just some random unknown system variables :-)
			{name: "Icinga", value: "Icinga"},     // unknown sys var
			{name: "IcingaDB", value: "IcingaDB"}, // unknown sys var
		},
		"VariablesWithCorrectValue": { // Setting system variables known by MySQL/MariaDB to a valid value
			{name: "transaction_isolation", value: "READ-UNCOMMITTED"},
			{name: "explain_format", value: "TRADITIONAL"},
			{name: "explain_format", value: "JSON"},
			{name: "completion_type", value: int64(1) /** CHAIN */},
			{name: "default_week_format", value: int64(7)},
		},
		"VariablesWithInvalidValues": { // System variables set to an invalid value
			{name: "transaction_isolation", value: "REPEATABLE-WRITE", expect: &mysql.MySQLError{Number: 1231}},
			{name: "completion_type", value: int64(-10), expect: &mysql.MySQLError{Number: 1231}}, // Min valid val 0
			{name: "completion_type", value: int64(10), expect: &mysql.MySQLError{Number: 1231}},  // Max valid val 2
			{name: "explain_format", value: "IcingaDB", expect: &mysql.MySQLError{Number: 1231}},
		},
	}

	rdb := it.MysqlDatabase()
	db, err := sqlx.Open(rdb.Driver(), rdb.DSN())
	require.NoError(t, err, "opening MySQL/MariaDB database should not fail")

	ctx := context.Background()
	for name, vs := range vars {
		t.Run(name, func(t *testing.T) {
			for _, v := range vs {
				conn, err := db.DB.Conn(ctx)
				assert.NoError(t, err, "connecting to MySQL/MariaDB database should not fail")

				err = conn.Raw(func(conn any) error {
					return setSessionVariableIfExists(ctx, conn.(driver.Conn), v.name, v.value)
				})

				assert.ErrorIsf(t, err, v.expect, "setting %q variable to '%v' returns unexpected result", v.name, v.value)
				assert.NoError(t, conn.Close(), "closing MySQL/MariaDB connection should not fail")
			}
		})
	}
}

