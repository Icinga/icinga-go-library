package database

import (
	"context"
	"database/sql/driver"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/icinga/icinga-go-library/types"
	"github.com/pkg/errors"
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
