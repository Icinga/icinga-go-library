package database

import (
	"context"
	"database/sql/driver"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/icinga/icinga-go-library/types"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"strings"
)

// CantPerformQuery wraps the given error with the specified query that cannot be executed.
func CantPerformQuery(err error, q string) error {
	return errors.Wrapf(err, "can't perform %q", q)
}

// TableName returns the table of t.
func TableName(t interface{}) string {
	if tn, ok := t.(TableNamer); ok {
		return tn.TableName()
	} else {
		return strcase.Snake(types.Name(t))
	}
}

// SplitOnDupId returns a state machine which tracks the inputs' IDs.
// Once an already seen input arrives, it demands splitting.
func SplitOnDupId[T IDer]() com.BulkChunkSplitPolicy[T] {
	seenIds := map[string]struct{}{}

	return func(ider T) bool {
		id := ider.ID().String()

		_, ok := seenIds[id]
		if ok {
			seenIds = map[string]struct{}{id: {}}
		} else {
			seenIds[id] = struct{}{}
		}

		return ok
	}
}

// InsertObtainID executes the given query and fetches the last inserted ID.
//
// Using this method for database tables that don't define an auto-incrementing ID, or none at all,
// will not work. The only supported column that can be retrieved with this method is id.
//
// This function expects [TxOrDB] as an executor of the provided query, and is usually a *[sqlx.Tx] or *[DB] instance.
//
// Returns the retrieved ID on success and error on any database inserting/retrieving failure.
func InsertObtainID(ctx context.Context, conn TxOrDB, stmt string, arg any) (int64, error) {
	var resultID int64
	switch conn.DriverName() {
	case PostgreSQL:
		stmt = stmt + " RETURNING id"
		query, args, err := conn.BindNamed(stmt, arg)
		if err != nil {
			return 0, errors.Wrapf(err, "can't bind named query %q", stmt)
		}

		if err := sqlx.GetContext(ctx, conn, &resultID, query, args...); err != nil {
			return 0, CantPerformQuery(err, query)
		}
	default:
		result, err := sqlx.NamedExecContext(ctx, conn, stmt, arg)
		if err != nil {
			return 0, CantPerformQuery(err, stmt)
		}

		resultID, err = result.LastInsertId()
		if err != nil {
			return 0, errors.Wrap(err, "can't retrieve last inserted ID")
		}
	}

	return resultID, nil
}

// BuildInsertStmtWithout builds an insert stmt without the provided column.
func BuildInsertStmtWithout(db *DB, into interface{}, withoutColumn string) string {
	columns := db.BuildColumns(into)
	for i, column := range columns {
		if column == withoutColumn {
			columns = append(columns[:i], columns[i+1:]...)
			break
		}
	}

	return fmt.Sprintf(
		`INSERT INTO "%s" ("%s") VALUES (%s)`,
		TableName(into), strings.Join(columns, `", "`),
		fmt.Sprintf(":%s", strings.Join(columns, ", :")),
	)
}

// unsafeSetSessionVariableIfExists sets the given MySQL/MariaDB system variable for the specified database session.
//
// NOTE: It is unsafe to use this function with untrusted/user supplied inputs and poses an SQL injection,
// because it doesn't use a prepared statement, but executes the SQL command directly with the provided inputs.
//
// When the "SET SESSION" command fails with "Unknown system variable (1193)", the error will be silently
// dropped but returns all other database errors.
func unsafeSetSessionVariableIfExists(ctx context.Context, conn driver.Conn, variable, value string) error {
	stmt := fmt.Sprintf("SET SESSION %s=%s", variable, value)

	if _, err := conn.(driver.ExecerContext).ExecContext(ctx, stmt, nil); err != nil {
		if errors.Is(err, &mysql.MySQLError{Number: 1193}) { // Unknown system variable
			return nil
		}

		return CantPerformQuery(err, stmt)
	}

	return nil
}

var (
	_ com.BulkChunkSplitPolicyFactory[Entity] = SplitOnDupId[Entity]
)
