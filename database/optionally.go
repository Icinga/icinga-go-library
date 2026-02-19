package database

import (
	"context"
	"github.com/icinga/icinga-go-library/com"
	"github.com/pkg/errors"
)

// QueryType represents the type of database query, expressed as an enum-like integer value.
type QueryType int

const (
	// SelectQuery represents a SQL SELECT query type, used for retrieving data from a database.
	SelectQuery QueryType = iota

	// InsertQuery represents the constant value for an INSERT database query.
	InsertQuery

	// UpsertQuery represents the constant value used for an UPSERT (INSERT or UPDATE) database query.
	UpsertQuery

	// UpdateQuery represents the constant value for an UPDATE database query.
	UpdateQuery

	// DeleteQuery represents the constant value for a DELETE query.
	DeleteQuery
)

// Queryable defines methods for bulk executing database entities such as upsert, insert, and update.
type Queryable interface {
	// Stream bulk executes database Entity(ies) for the following three database query types.
	// 	* Upsert - Stream consumes from the provided entities channel and bulk upserts them via DB.NamedBulkExec.
	//	  If not explicitly specified via WithStatement, the upsert statement is generated dynamically via the
	//	  QueryBuilder. The bulk size is controlled via Options.MaxPlaceholdersPerStatement and concurrency
	//	  via the Options.MaxConnectionsPerTable.
	//	* Insert(Ignore) - Stream does likewise for insert statement and bulk inserts the entities via DB.NamedBulkExec.
	// 	  If not explicitly specified via WithStatement, the insert statement is generated dynamically via the
	//	  QueryBuilder. The bulk size is controlled via Options.MaxPlaceholdersPerStatement and concurrency
	//	  via the Options.MaxConnectionsPerTable.
	//	* Update - Stream bulk updates the entities via DB.NamedBulkExecTx. If not explicitly specified via
	//	  WithStatement, the update statement is generated dynamically via the QueryBuilder. The bulk size is
	//	  controlled via Options.MaxRowsPerTransaction and concurrency via the Options.MaxConnectionsPerTable.
	// Entities for which the query ran successfully will be passed to the onSuccess handlers (if provided).
	Stream(ctx context.Context, entities <-chan Entity, onSuccess ...OnSuccess[Entity]) error

	// StreamAny bulk executes the streamed items of type any using the [DB.BulkExec] method.
	StreamAny(ctx context.Context, args <-chan any, onSuccess ...OnSuccess[any]) error
}

// NewSelect initializes a new Queryable object of type SelectQuery for a given [DB], subject.
func NewSelect(db *DB, subject any, options ...QueryableOption) Queryable {
	return newQuery(db, subject, append([]QueryableOption{withSetQueryType(SelectQuery)}, options...)...)
}

// NewInsert initializes a new Queryable object of type InsertQuery for a given [DB], subject.
func NewInsert(db *DB, subject any, options ...QueryableOption) Queryable {
	return newQuery(db, subject, append([]QueryableOption{withSetQueryType(InsertQuery)}, options...)...)
}

// NewUpsert initializes a new Queryable object of type UpsertQuery for a given [DB], subject.
func NewUpsert(db *DB, subject any, options ...QueryableOption) Queryable {
	return newQuery(db, subject, append([]QueryableOption{withSetQueryType(UpsertQuery)}, options...)...)
}

// NewUpdate initializes a new Queryable object of type UpdateQuery for a given [DB], subject.
func NewUpdate(db *DB, subject any, options ...QueryableOption) Queryable {
	return newQuery(db, subject, append([]QueryableOption{withSetQueryType(UpdateQuery)}, options...)...)
}

// NewDelete initializes a new Queryable object of type DeleteQuery for a given [DB], subject.
func NewDelete(db *DB, subject any, options ...QueryableOption) Queryable {
	return newQuery(db, subject, append([]QueryableOption{withSetQueryType(DeleteQuery)}, options...)...)
}

// queryable represents a database query type with customizable behavior for dynamic and static SQL statements.
type queryable struct {
	db *DB

	// qb is the query builder used to construct SQL statements for various database
	// statements if, and only if stmt is not set.
	qb *QueryBuilder

	// qtype defines the type of database query (e.g., SELECT, INSERT) to perform, influencing query construction behavior.
	qtype QueryType

	// scoper is used to dynamically generate scoped database queries if, and only if stmt is not set.
	scoper any

	// stmt is used to cache statically provided database statements.
	stmt string

	// placeholders is used to determine the entities bulk/chunk size for statically provided statements.
	placeholders int

	// ignoreOnError is only used to generate special insert statements that silently suppress duplicate key errors.
	ignoreOnError bool
}

// Assert that *queryable type satisfies the Queryable interface.
var _ Queryable = (*queryable)(nil)

// Stream implements the [Queryable.Stream] method.
func (q *queryable) Stream(ctx context.Context, entities <-chan Entity, onSuccess ...OnSuccess[Entity]) error {
	sem := q.db.GetSemaphoreForTable(TableName(q.qb.subject))
	stmt, placeholders := q.buildStmt()
	batchSize := q.db.BatchSizeByPlaceholders(placeholders)

	switch q.qtype {
	case SelectQuery: // TODO: support select statements?
	case InsertQuery:
		return q.db.NamedBulkExec(ctx, stmt, batchSize, sem, entities, com.NeverSplit[Entity], onSuccess...)
	case UpsertQuery:
		return q.db.NamedBulkExec(ctx, stmt, batchSize, sem, entities, SplitOnDupId[Entity], onSuccess...)
	case UpdateQuery:
		return q.db.NamedBulkExecTx(ctx, stmt, q.db.Options.MaxRowsPerTransaction, sem, entities)
	case DeleteQuery:
		return errors.Errorf("can't stream entities for 'DELETE' query")
	}

	return errors.Errorf("unsupported query type: %v", q.qtype)
}

// StreamAny implements the [Queryable.StreamAny] method.
func (q *queryable) StreamAny(ctx context.Context, args <-chan any, onSuccess ...OnSuccess[any]) error {
	stmt, _ := q.buildStmt()
	sem := q.db.GetSemaphoreForTable(TableName(q.qb.subject))

	return q.db.BulkExec(ctx, stmt, q.db.Options.MaxPlaceholdersPerStatement, sem, args, onSuccess...)
}

// buildStmt constructs the SQL statement based on the type of query (Select, Insert, Upsert, Update, Delete).
// It also determines the number of placeholders to be used in the statement.
func (q *queryable) buildStmt() (string, int) {
	if q.stmt != "" {
		return q.stmt, q.placeholders
	}

	var stmt string
	var placeholders int

	switch q.qtype {
	case SelectQuery: // TODO: support select statements?
	case InsertQuery:
		if q.ignoreOnError {
			stmt, placeholders = q.qb.InsertIgnore(q.db)
		} else {
			stmt, placeholders = q.qb.Insert(q.db)
		}
	case UpsertQuery:
		if q.stmt != "" {
			stmt, placeholders = q.stmt, q.placeholders
		} else {
			stmt, placeholders = q.qb.Upsert(q.db)
		}
	case UpdateQuery:
		stmt = q.stmt
		if stmt == "" {
			if q.scoper != nil && q.scoper.(string) != "" {
				stmt, _ = q.qb.UpdateScoped(q.db, q.scoper)
			} else {
				stmt, _ = q.qb.Update(q.db)
			}
		}
	case DeleteQuery:
		if q.stmt != "" {
			stmt, placeholders = q.stmt, q.placeholders
		} else if q.scoper != "" {
			stmt = q.qb.DeleteBy(q.scoper.(string))
		} else {
			stmt = q.qb.Delete()
		}
	}

	return stmt, placeholders
}

// newQuery initializes a new Queryable object for a given [DB], subject, and query type.
// It also applies optional query options to the just created queryable object.
//
// Note: If the query type is not explicitly set using WithSetQueryType, it will default to SELECT queries.
func newQuery(db *DB, subject any, options ...QueryableOption) Queryable {
	q := &queryable{db: db, qb: &QueryBuilder{subject: subject}}
	for _, option := range options {
		option(q)
	}

	return q
}

// QueryableOption describes the base functional specification for all the queryable types.
type QueryableOption func(*queryable)

// withSetQueryType sets the type of database query to be executed/generated.
func withSetQueryType(qtype QueryType) QueryableOption { return func(q *queryable) { q.qtype = qtype } }

// WithStatement configures a static SQL statement and its associated placeholders for a queryable entity.
//
// Note that using WithStatement always suppresses all other available queryable options and unlike
// some other options, this can be used to explicitly provide a custom query for all kinds of DB stmts.
//
// Returns a function that lazily modifies a given queryable type by setting its stmt and placeholders fields.
func WithStatement(stmt string, placeholders int) QueryableOption {
	return func(q *queryable) {
		q.stmt = stmt
		q.placeholders = placeholders
	}
}

// WithColumns statically configures the DB columns to be used for building the database statements.
//
// Setting the queryable columns while using WithStatement has no behavioural effects, thus these columns are never
// used. Additionally, for upsert statements, WithColumns not only defines the columns to be actually inserted but
// the columns to be updated when a duplicate key error occurs as well. However, to maintain the compatibility with
// legacy implementations, a query subject that implements the Upserter interface takes a higher precedence over
// those explicitly set columns for the "update on duplicate key error" part.
//
// Note that using this option for Delete statements has no effect as well, hence its usage is discouraged.
//
// Returns a function that lazily modifies a given queryable type by setting its columns.
func WithColumns(columns ...string) QueryableOption {
	return func(q *queryable) { q.qb.SetColumns(columns...) }
}

// WithoutColumns returns a QueryableOption callback that excludes the DB columns from the generated DB statements.
//
// Setting the excludable columns while using WithStatement has no behavioural effects, so these columns may or may
// not be excluded depending on the explicitly set statement. Also, note that using this option for Delete statements
// has no effect as well, hence its usage is prohibited.
func WithoutColumns(columns ...string) QueryableOption {
	return func(q *queryable) { q.qb.SetExcludedColumns(columns...) }
}

// WithIgnoreOnError returns a InsertOption callback that sets the ignoreOnError flag DB insert statements.
//
// When this flag is set, the dynamically generated insert statement will cause to suppress all duplicate key errors.
//
// Setting this flag while using WithStatement has no behavioural effects, so the final database statement
// may or may not silently suppress "duplicate key errors" depending on the explicitly set statement.
func WithIgnoreOnError() QueryableOption { return func(q *queryable) { q.ignoreOnError = true } }

// WithByColumn returns a functional option for DeleteOption or UpdateOption, setting the scoper to the provided column.
func WithByColumn(column string) QueryableOption {
	return func(q *queryable) { q.scoper = column }
}
