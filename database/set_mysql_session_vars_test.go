package database

import (
	"context"
	"database/sql/driver"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
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
			{name: "autocommit", value: true},
			{name: "binlog_format", value: "MIXED"},
			{name: "completion_type", value: int64(1) /** CHAIN */},
			{name: "default_storage_engine", value: "InnoDB"},
		},
		"VariablesWithInvalidValues": { // System variables set to an invalid value
			{name: "autocommit", value: "NOT-TRUE", expect: &mysql.MySQLError{Number: 1231}},
			{name: "binlog_format", value: "IcingaDB", expect: &mysql.MySQLError{Number: 1231}},          // Invalid val!
			{name: "completion_type", value: int64(-10), expect: &mysql.MySQLError{Number: 1231}},        // Min valid val 0
			{name: "default_storage_engine", value: "IcingaDB", expect: &mysql.MySQLError{Number: 1286}}, // Unknown storage Engine!
		},
	}

	ctx := context.Background()
	db := GetTestDB(ctx, t, "ICINGAGOLIBRARY")
	if db.DriverName() == PostgreSQL {
		t.Skipf("skipping set session vars test for %q driver", PostgreSQL)
	}

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
