package database

import (
	"context"
	"database/sql/driver"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/icinga/icinga-go-library/types"
	"github.com/jmoiron/sqlx"
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

// InsertObtainID executes the given query and fetches the last inserted ID.
//
// Using this method for database tables that don't define an auto-incrementing ID, or none at all,
// will not work. The only supported column that can be retrieved with this method is id.
// This function expects [TxOrDB] as an executor of the provided query, and is usually a *[sqlx.Tx] or *[DB] instance.
// Returns the retrieved ID on success and error on any database inserting/retrieving failure.
func InsertObtainID(ctx context.Context, conn TxOrDB, stmt string, arg any) (int64, error) {
	var resultID int64
	switch conn.DriverName() {
	case PostgreSQL:
		query := stmt + " RETURNING id"
		ps, err := conn.PrepareNamedContext(ctx, query)
		if err != nil {
			return 0, errors.Wrapf(err, "cannot prepare %q", query)
		}
		// We're deferring the ps#Close invocation here just to be on the safe side, otherwise it's
		// closed manually later on and the error is handled gracefully (if any).
		defer func() { _ = ps.Close() }()

		if err := ps.GetContext(ctx, &resultID, arg); err != nil {
			return 0, CantPerformQuery(err, query)
		}

		if err := ps.Close(); err != nil {
			return 0, errors.Wrapf(err, "cannot close prepared statement %q", query)
		}
	default:
		result, err := sqlx.NamedExecContext(ctx, conn, stmt, arg)
		if err != nil {
			return 0, CantPerformQuery(err, stmt)
		}

		resultID, err = result.LastInsertId()
		if err != nil {
			return 0, errors.Wrap(err, "cannot retrieve last inserted ID")
		}
	}

	return resultID, nil
}

// setGaleraOpts sets the "wsrep_sync_wait" variable for each session ensures that causality checks are performed
// before execution and that each statement is executed on a fully synchronized node. Doing so prevents foreign key
// violation when inserting into dependent tables on different MariaDB/MySQL nodes. When using MySQL single nodes,
// the "SET SESSION" command will fail with "Unknown system variable (1193)" and will therefore be silently dropped.
//
// https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_sync_wait
func setGaleraOpts(ctx context.Context, conn driver.Conn, wsrepSyncWait int64) error {
	const galeraOpts = "SET SESSION wsrep_sync_wait=?"

	stmt, err := conn.(driver.ConnPrepareContext).PrepareContext(ctx, galeraOpts)
	if err != nil {
		if errors.Is(err, &mysql.MySQLError{Number: 1193}) { // Unknown system variable
			return nil
		}

		return errors.Wrap(err, "cannot prepare "+galeraOpts)
	}
	// This is just for an unexpected exit and any returned error can safely be ignored and in case
	// of the normal function exit, the stmt is closed manually, and its error is handled gracefully.
	defer func() { _ = stmt.Close() }()

	_, err = stmt.(driver.StmtExecContext).ExecContext(ctx, []driver.NamedValue{{Value: wsrepSyncWait}})
	if err != nil {
		return errors.Wrap(err, "cannot execute "+galeraOpts)
	}

	if err = stmt.Close(); err != nil {
		return errors.Wrap(err, "cannot close prepared statement "+galeraOpts)
	}

	return nil
}

var (
	_ com.BulkChunkSplitPolicyFactory[Entity] = SplitOnDupId[Entity]
)
