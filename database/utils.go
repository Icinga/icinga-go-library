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

// setSessionVariableIfExists sets the given MySQL/MariaDB system variable for the specified database session.
//
// When the "SET SESSION" command fails with "Unknown system variable (1193)", the error will be silently dropped but
// returns all other database errors.
func setSessionVariableIfExists(ctx context.Context, conn driver.Conn, variable string, value any) error {
	query := fmt.Sprintf("SET SESSION %s=?", variable)

	stmt, err := conn.(driver.ConnPrepareContext).PrepareContext(ctx, query)
	if err != nil {
		if errors.Is(err, &mysql.MySQLError{Number: 1193}) { // Unknown system variable
			return nil
		}

		return errors.Wrap(err, "cannot prepare "+query)
	}
	// This is just for an unexpected exit and any returned error can safely be ignored and in case
	// of the normal function exit, the stmt is closed manually, and its error is handled gracefully.
	defer func() { _ = stmt.Close() }()

	_, err = stmt.(driver.StmtExecContext).ExecContext(ctx, []driver.NamedValue{{Value: value}})
	if err != nil {
		return errors.Wrap(err, "cannot execute "+query)
	}

	if err = stmt.Close(); err != nil {
		return errors.Wrap(err, "cannot close prepared statement "+query)
	}

	return nil
}

var (
	_ com.BulkChunkSplitPolicyFactory[Entity] = SplitOnDupId[Entity]
)
