package database

import (
	"context"
	"database/sql"
	goErrors "errors"
	"fmt"
	"github.com/pkg/errors"
)

// SchemaUpgrade represents a single upgrade step.
type SchemaUpgrade struct {
	// Version specifies the target version as in the schema table.
	Version string
	// DDL aggregates one or more .sql files' contents.
	DDL []string
}

// SchemaData summaries all available DDL for a database type.
type SchemaData struct {
	// Schema aggregates one or more .sql files' contents.
	Schema []string
	// Upgrades aggregates all available upgrade steps in ascending order.
	Upgrades []SchemaUpgrade
}

var ErrDbTypeNotUpgradable = goErrors.New("no schema supplied for given database type")

// AutoUpgradeSchema imports or upgrades the schema in db from schemaData.
func AutoUpgradeSchema(
	ctx context.Context, db *DB, dbName, schemaTable, schemaTableVersionColumn, schemaTableTimestampColumn string,
	schemaData map[string]SchemaData,
) error {
	ourSchema, driverSupported := schemaData[db.DriverName()]
	if !driverSupported {
		return errors.Wrap(ErrDbTypeNotUpgradable, "can't upgrade schema")
	}

	err := db.QueryRowContext(
		ctx, db.Rebind("SELECT 1 FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=?"),
		dbName, schemaTable,
	).Scan(new(int8))

	switch err {
	case nil:
		var currentVersion string

		err := db.QueryRowContext(ctx, fmt.Sprintf(
			"SELECT %s FROM %s ORDER BY %s DESC LIMIT 1",
			schemaTableVersionColumn, schemaTable, schemaTableTimestampColumn,
		)).Scan(&currentVersion)
		if err != nil {
			return errors.Wrap(err, "can't check schema version")
		}

		// If there's no upgrade step to the current version, it must be the first one, so apply all upgrades.
		upgrades := ourSchema.Upgrades

		for i, upgrade := range ourSchema.Upgrades {
			if upgrade.Version == currentVersion {
				// If there's an upgrade step to the current version, apply all subsequent ones.
				upgrades = ourSchema.Upgrades[i+1:]
				break
			}
		}

		for _, upgrade := range upgrades {
			if err := importSchema(ctx, db, upgrade.DDL); err != nil {
				return errors.Wrap(err, "can't upgrade schema")
			}
		}

		return nil
	case sql.ErrNoRows:
		return errors.Wrap(importSchema(ctx, db, ourSchema.Schema), "can't import schema")
	default:
		return errors.Wrap(err, "can't check schema existence")
	}
}

// importSchema imports one or more .sql files' contents from schema into db.
func importSchema(ctx context.Context, db *DB, schema []string) error {
	for _, ddls := range schema {
		for _, ddl := range MysqlSplitStatements(ddls) {
			if _, err := db.ExecContext(ctx, ddl); err != nil {
				return err
			}
		}
	}

	return nil
}
