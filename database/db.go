package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/periodic"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/strcase"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/reflectx"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DB is a wrapper around sqlx.DB with bulk execution,
// statement building, streaming and logging capabilities.
type DB struct {
	*sqlx.DB

	Options *Options

	addr              string
	columnMap         ColumnMap
	logger            *logging.Logger
	tableSemaphores   map[string]*semaphore.Weighted
	tableSemaphoresMu sync.Mutex
}

// Options define user configurable database options.
type Options struct {
	// Maximum number of open connections to the database.
	MaxConnections int `yaml:"max_connections" env:"MAX_CONNECTIONS" default:"16"`

	// Maximum number of connections per table,
	// regardless of what the connection is actually doing,
	// e.g. INSERT, UPDATE, DELETE.
	MaxConnectionsPerTable int `yaml:"max_connections_per_table" env:"MAX_CONNECTIONS_PER_TABLE" default:"8"`

	// MaxPlaceholdersPerStatement defines the maximum number of placeholders in an
	// INSERT, UPDATE or DELETE statement. Theoretically, MySQL can handle up to 2^16-1 placeholders,
	// but this increases the execution time of queries and thus reduces the number of queries
	// that can be executed in parallel in a given time.
	// The default is 2^13, which in our tests showed the best performance in terms of execution time and parallelism.
	MaxPlaceholdersPerStatement int `yaml:"max_placeholders_per_statement" env:"MAX_PLACEHOLDERS_PER_STATEMENT" default:"8192"`

	// MaxRowsPerTransaction defines the maximum number of rows per transaction.
	// The default is 2^13, which in our tests showed the best performance in terms of execution time and parallelism.
	MaxRowsPerTransaction int `yaml:"max_rows_per_transaction" env:"MAX_ROWS_PER_TRANSACTION" default:"8192"`

	// WsrepSyncWait enforces Galera cluster nodes to perform strict cluster-wide causality checks
	// before executing specific SQL queries determined by the number you provided.
	// Please refer to the below link for a detailed description.
	// https://icinga.com/docs/icinga-db/latest/doc/03-Configuration/#galera-cluster
	WsrepSyncWait int `yaml:"wsrep_sync_wait" env:"WSREP_SYNC_WAIT" default:"7"`
}

// Validate checks constraints in the supplied database options and returns an error if they are violated.
func (o *Options) Validate() error {
	if o.MaxConnections == 0 {
		return errors.New("max_connections cannot be 0. Configure a value greater than zero, or use -1 for no connection limit")
	}
	if o.MaxConnectionsPerTable < 1 {
		return errors.New("max_connections_per_table must be at least 1")
	}
	if o.MaxPlaceholdersPerStatement < 1 {
		return errors.New("max_placeholders_per_statement must be at least 1")
	}
	if o.MaxRowsPerTransaction < 1 {
		return errors.New("max_rows_per_transaction must be at least 1")
	}
	if o.WsrepSyncWait < 0 || o.WsrepSyncWait > 15 {
		return errors.New("wsrep_sync_wait can only be set to a number between 0 and 15")
	}

	return nil
}

// NewDbFromConfig returns a new DB from Config.
func NewDbFromConfig(c *Config, logger *logging.Logger, connectorCallbacks RetryConnectorCallbacks) (*DB, error) {
	var addr string
	var db *sqlx.DB

	switch c.Type {
	case "mysql":
		config := mysql.NewConfig()

		config.User = c.User
		config.Passwd = c.Password
		config.Logger = MysqlFuncLogger(logger.Debug)

		if utils.IsUnixAddr(c.Host) {
			config.Net = "unix"
			config.Addr = c.Host
			addr = "(" + config.Addr + ")"
		} else {
			config.Net = "tcp"
			port := c.Port
			if port == 0 {
				port = 3306
			}
			config.Addr = net.JoinHostPort(c.Host, fmt.Sprint(port))
			addr = config.Addr
		}

		config.DBName = c.Database
		config.Timeout = time.Minute
		config.Params = map[string]string{"sql_mode": "'TRADITIONAL,ANSI_QUOTES'"}

		tlsConfig, err := c.TlsOptions.MakeConfig(c.Host)
		if err != nil {
			return nil, err
		}

		config.TLS = tlsConfig

		connector, err := mysql.NewConnector(config)
		if err != nil {
			return nil, errors.Wrap(err, "can't open mysql database")
		}

		onInitConn := connectorCallbacks.OnInitConn
		connectorCallbacks.OnInitConn = func(ctx context.Context, conn driver.Conn) error {
			if onInitConn != nil {
				if err := onInitConn(ctx, conn); err != nil {
					return err
				}
			}

			// Set the "wsrep_sync_wait" variable for each session and ensures that causality checks are performed
			// before execution and that each statement is executed on a fully synchronized node. Doing so prevents
			// foreign key violation when inserting into dependent tables on different MariaDB/MySQL nodes. When using
			// MySQL single nodes, the "SET SESSION" command will fail with "Unknown system variable (1193)" and will
			// therefore be silently dropped.
			// https://mariadb.com/kb/en/galera-cluster-system-variables/#wsrep_sync_wait
			return unsafeSetSessionVariableIfExists(ctx, conn, "wsrep_sync_wait", fmt.Sprint(c.Options.WsrepSyncWait))
		}

		db = sqlx.NewDb(sql.OpenDB(NewConnector(connector, logger, connectorCallbacks)), MySQL)
	case "pgsql":
		uri := &url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(c.User, c.Password),
			Path:   "/" + url.PathEscape(c.Database),
		}

		query := url.Values{
			"connect_timeout":   {"60"},
			"binary_parameters": {"yes"},

			// Host and port can alternatively be specified in the query string. lib/pq can't parse the connection URI
			// if a Unix domain socket path is specified in the host part of the URI, therefore always use the query
			// string. See also https://github.com/lib/pq/issues/796
			"host": {c.Host},
		}

		port := c.Port
		if port == 0 {
			port = 5432
		}
		query.Set("port", strconv.FormatInt(int64(port), 10))

		if _, err := c.TlsOptions.MakeConfig(c.Host); err != nil {
			return nil, err
		}

		if c.TlsOptions.Enable {
			if c.TlsOptions.Insecure {
				query.Set("sslmode", "require")
			} else {
				query.Set("sslmode", "verify-full")
			}

			if c.TlsOptions.Cert != "" {
				query.Set("sslcert", c.TlsOptions.Cert)
			}

			if c.TlsOptions.Key != "" {
				query.Set("sslkey", c.TlsOptions.Key)
			}

			if c.TlsOptions.Ca != "" {
				query.Set("sslrootcert", c.TlsOptions.Ca)
			}
		} else {
			query.Set("sslmode", "disable")
		}

		uri.RawQuery = query.Encode()

		connector, err := pq.NewConnector(uri.String())
		if err != nil {
			return nil, errors.Wrap(err, "can't open pgsql database")
		}

		if utils.IsUnixAddr(c.Host) {
			// https://www.postgresql.org/docs/17/runtime-config-connection.html#GUC-UNIX-SOCKET-DIRECTORIES
			addr = fmt.Sprintf("(%s/.s.PGSQL.%d)", strings.TrimRight(c.Host, "/"), port)
		} else {
			addr = utils.JoinHostPort(c.Host, port)
		}
		db = sqlx.NewDb(sql.OpenDB(NewConnector(connector, logger, connectorCallbacks)), PostgreSQL)
	default:
		return nil, unknownDbType(c.Type)
	}

	if c.TlsOptions.Enable {
		addr = fmt.Sprintf("%s+tls://%s@%s/%s", c.Type, c.User, addr, c.Database)
	} else {
		addr = fmt.Sprintf("%s://%s@%s/%s", c.Type, c.User, addr, c.Database)
	}

	db.SetMaxIdleConns(c.Options.MaxConnections / 3)
	db.SetMaxOpenConns(c.Options.MaxConnections)

	db.Mapper = reflectx.NewMapperFunc("db", strcase.Snake)

	return &DB{
		DB:              db,
		Options:         &c.Options,
		columnMap:       NewColumnMap(db.Mapper),
		addr:            addr,
		logger:          logger,
		tableSemaphores: make(map[string]*semaphore.Weighted),
	}, nil
}

// GetAddr returns a URI-like database connection string.
//
// It has the following syntax:
//
//	type[+tls]://user@host[:port]/database
func (db *DB) GetAddr() string {
	return db.addr
}

// MarshalLogObject implements [zapcore.ObjectMarshaler], adding the database address [DB.GetAddr] to each log message.
func (db *DB) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddString("database_address", db.GetAddr())

	return nil
}

// BuildColumns returns all columns of the given struct.
func (db *DB) BuildColumns(subject interface{}) []string {
	return slices.Clone(db.columnMap.Columns(subject))
}

// BuildDeleteStmt returns a DELETE statement for the given struct.
func (db *DB) BuildDeleteStmt(from interface{}) string {
	return fmt.Sprintf(
		`DELETE FROM "%s" WHERE id IN (?)`,
		TableName(from),
	)
}

// BuildInsertStmt returns an INSERT INTO statement for the given struct.
func (db *DB) BuildInsertStmt(into interface{}) (string, int) {
	columns := db.columnMap.Columns(into)

	return fmt.Sprintf(
		`INSERT INTO "%s" ("%s") VALUES (%s)`,
		TableName(into),
		strings.Join(columns, `", "`),
		fmt.Sprintf(":%s", strings.Join(columns, ", :")),
	), len(columns)
}

// BuildInsertIgnoreStmt returns an INSERT statement for the specified struct for
// which the database ignores rows that have already been inserted.
func (db *DB) BuildInsertIgnoreStmt(into interface{}) (string, int) {
	table := TableName(into)
	columns := db.columnMap.Columns(into)
	var clause string

	switch db.DriverName() {
	case MySQL:
		// MySQL treats UPDATE id = id as a no-op.
		clause = fmt.Sprintf(`ON DUPLICATE KEY UPDATE "%s" = "%s"`, columns[0], columns[0])
	case PostgreSQL:
		var constraint string
		if constrainter, ok := into.(PgsqlOnConflictConstrainter); ok {
			constraint = constrainter.PgsqlOnConflictConstraint()
		} else {
			constraint = "pk_" + table
		}

		clause = fmt.Sprintf("ON CONFLICT ON CONSTRAINT %s DO NOTHING", constraint)
	}

	return fmt.Sprintf(
		`INSERT INTO "%s" ("%s") VALUES (%s) %s`,
		table,
		strings.Join(columns, `", "`),
		fmt.Sprintf(":%s", strings.Join(columns, ", :")),
		clause,
	), len(columns)
}

// BuildSelectStmt returns a SELECT query that creates the FROM part from the given table struct
// and the column list from the specified columns struct.
func (db *DB) BuildSelectStmt(table interface{}, columns interface{}) string {
	q := fmt.Sprintf(
		`SELECT "%s" FROM "%s"`,
		strings.Join(db.columnMap.Columns(columns), `", "`),
		TableName(table),
	)

	if scoper, ok := table.(Scoper); ok {
		where, _ := db.BuildWhere(scoper.Scope())
		q += ` WHERE ` + where
	}

	return q
}

// BuildUpdateStmt returns an UPDATE statement for the given struct.
func (db *DB) BuildUpdateStmt(update interface{}) (string, int) {
	columns := db.columnMap.Columns(update)
	set := make([]string, 0, len(columns))

	for _, col := range columns {
		set = append(set, fmt.Sprintf(`"%s" = :%s`, col, col))
	}

	return fmt.Sprintf(
		`UPDATE "%s" SET %s WHERE id = :id`,
		TableName(update),
		strings.Join(set, ", "),
	), len(columns) + 1 // +1 because of WHERE id = :id
}

// BuildUpsertStmt returns an upsert statement for the given struct.
func (db *DB) BuildUpsertStmt(subject interface{}) (stmt string, placeholders int) {
	insertColumns := db.columnMap.Columns(subject)
	table := TableName(subject)
	var updateColumns []string

	if upserter, ok := subject.(Upserter); ok {
		updateColumns = db.columnMap.Columns(upserter.Upsert())
	} else {
		updateColumns = insertColumns
	}

	var clause, setFormat string
	switch db.DriverName() {
	case MySQL:
		clause = "ON DUPLICATE KEY UPDATE"
		setFormat = `"%[1]s" = VALUES("%[1]s")`
	case PostgreSQL:
		var constraint string
		if constrainter, ok := subject.(PgsqlOnConflictConstrainter); ok {
			constraint = constrainter.PgsqlOnConflictConstraint()
		} else {
			constraint = "pk_" + table
		}

		clause = fmt.Sprintf("ON CONFLICT ON CONSTRAINT %s DO UPDATE SET", constraint)
		setFormat = `"%[1]s" = EXCLUDED."%[1]s"`
	}

	set := make([]string, 0, len(updateColumns))

	for _, col := range updateColumns {
		set = append(set, fmt.Sprintf(setFormat, col))
	}

	return fmt.Sprintf(
		`INSERT INTO "%s" ("%s") VALUES (%s) %s %s`,
		table,
		strings.Join(insertColumns, `", "`),
		fmt.Sprintf(":%s", strings.Join(insertColumns, ",:")),
		clause,
		strings.Join(set, ","),
	), len(insertColumns)
}

// BuildWhere returns a WHERE clause with named placeholder conditions built from the specified struct
// combined with the AND operator.
func (db *DB) BuildWhere(subject interface{}) (string, int) {
	columns := db.columnMap.Columns(subject)
	where := make([]string, 0, len(columns))
	for _, col := range columns {
		where = append(where, fmt.Sprintf(`"%s" = :%s`, col, col))
	}

	return strings.Join(where, ` AND `), len(columns)
}

// OnSuccess is a callback for successful (bulk) DML operations.
type OnSuccess[T any] func(ctx context.Context, affectedRows []T) (err error)

func OnSuccessIncrement[T any](counter *com.Counter) OnSuccess[T] {
	return func(_ context.Context, rows []T) error {
		counter.Add(uint64(len(rows)))
		return nil
	}
}

func OnSuccessSendTo[T any](ch chan<- T) OnSuccess[T] {
	return func(ctx context.Context, rows []T) error {
		for _, row := range rows {
			select {
			case ch <- row:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return nil
	}
}

// BulkExec bulk executes queries with a single slice placeholder in the form of `IN (?)`.
// Takes in up to the number of arguments specified in count from the arg stream,
// derives and expands a query and executes it with this set of arguments until the arg stream has been processed.
// The derived queries are executed in a separate goroutine with a weighting of 1
// and can be executed concurrently to the extent allowed by the semaphore passed in sem.
// Arguments for which the query ran successfully will be passed to onSuccess.
func (db *DB) BulkExec(
	ctx context.Context, query string, count int, sem *semaphore.Weighted, arg <-chan any, onSuccess ...OnSuccess[any],
) error {
	var counter com.Counter
	defer db.Log(ctx, query, &counter).Stop()

	g, ctx := errgroup.WithContext(ctx)
	// Use context from group.
	bulk := com.Bulk(ctx, arg, count, com.NeverSplit[any])

	g.Go(func() error {
		g, ctx := errgroup.WithContext(ctx)

		for b := range bulk {
			if err := sem.Acquire(ctx, 1); err != nil {
				return errors.Wrap(err, "can't acquire semaphore")
			}

			g.Go(func(b []interface{}) func() error {
				return func() error {
					defer sem.Release(1)

					return retry.WithBackoff(
						ctx,
						func(context.Context) error {
							stmt, args, err := sqlx.In(query, b)
							if err != nil {
								return errors.Wrapf(err, "can't build placeholders for %q", query)
							}

							stmt = db.Rebind(stmt)
							_, err = db.ExecContext(ctx, stmt, args...)
							if err != nil {
								return CantPerformQuery(err, query)
							}

							counter.Add(uint64(len(b)))

							for _, onSuccess := range onSuccess {
								if err := onSuccess(ctx, b); err != nil {
									return err
								}
							}

							return nil
						},
						retry.Retryable,
						backoff.DefaultBackoff,
						db.GetDefaultRetrySettings(),
					)
				}
			}(b))
		}

		return g.Wait()
	})

	return g.Wait()
}

// NamedBulkExec bulk executes queries with named placeholders in a VALUES clause most likely
// in the format INSERT ... VALUES. Takes in up to the number of entities specified in count
// from the arg stream, derives and executes a new query with the VALUES clause expanded to
// this set of arguments, until the arg stream has been processed.
// The queries are executed in a separate goroutine with a weighting of 1
// and can be executed concurrently to the extent allowed by the semaphore passed in sem.
// Entities for which the query ran successfully will be passed to onSuccess.
func (db *DB) NamedBulkExec(
	ctx context.Context, query string, count int, sem *semaphore.Weighted, arg <-chan Entity,
	splitPolicyFactory com.BulkChunkSplitPolicyFactory[Entity], onSuccess ...OnSuccess[Entity],
) error {
	var counter com.Counter
	defer db.Log(ctx, query, &counter).Stop()

	g, ctx := errgroup.WithContext(ctx)
	bulk := com.Bulk(ctx, arg, count, splitPolicyFactory)

	g.Go(func() error {
		for {
			select {
			case b, ok := <-bulk:
				if !ok {
					return nil
				}

				if err := sem.Acquire(ctx, 1); err != nil {
					return errors.Wrap(err, "can't acquire semaphore")
				}

				g.Go(func(b []Entity) func() error {
					return func() error {
						defer sem.Release(1)

						return retry.WithBackoff(
							ctx,
							func(ctx context.Context) error {
								_, err := db.NamedExecContext(ctx, query, b)
								if err != nil {
									return CantPerformQuery(err, query)
								}

								counter.Add(uint64(len(b)))

								for _, onSuccess := range onSuccess {
									if err := onSuccess(ctx, b); err != nil {
										return err
									}
								}

								return nil
							},
							retry.Retryable,
							backoff.DefaultBackoff,
							db.GetDefaultRetrySettings(),
						)
					}
				}(b))
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	return g.Wait()
}

// NamedBulkExecTx bulk executes queries with named placeholders in separate transactions.
// Takes in up to the number of entities specified in count from the arg stream and
// executes a new transaction that runs a new query for each entity in this set of arguments,
// until the arg stream has been processed.
//
// The transactions are executed in a separate goroutine with a weighting of 1
// and can be executed concurrently to the extent allowed by the semaphore passed in sem.
//
// Note that committing the transaction may not honor the context provided, as described further in [DB.ExecTx].
func (db *DB) NamedBulkExecTx(
	ctx context.Context, query string, count int, sem *semaphore.Weighted, arg <-chan Entity,
) error {
	var counter com.Counter
	defer db.Log(ctx, query, &counter).Stop()

	g, ctx := errgroup.WithContext(ctx)
	bulk := com.Bulk(ctx, arg, count, com.NeverSplit[Entity])

	g.Go(func() error {
		for {
			select {
			case b, ok := <-bulk:
				if !ok {
					return nil
				}

				if err := sem.Acquire(ctx, 1); err != nil {
					return errors.Wrap(err, "can't acquire semaphore")
				}

				g.Go(func(b []Entity) func() error {
					return func() error {
						defer sem.Release(1)

						return retry.WithBackoff(
							ctx,
							func(ctx context.Context) error {
								tx, err := db.BeginTxx(ctx, nil)
								if err != nil {
									return errors.Wrap(err, "can't start transaction")
								}
								defer func() { _ = tx.Rollback() }()

								stmt, err := tx.PrepareNamedContext(ctx, query)
								if err != nil {
									return errors.Wrap(err, "can't prepare named statement with context in transaction")
								}
								defer func() { _ = stmt.Close() }()

								for _, arg := range b {
									if _, err := stmt.ExecContext(ctx, arg); err != nil {
										return errors.Wrap(err, "can't execute statement in transaction")
									}
								}

								if err := tx.Commit(); err != nil {
									return errors.Wrap(err, "can't commit transaction")
								}

								counter.Add(uint64(len(b)))

								return nil
							},
							retry.Retryable,
							backoff.DefaultBackoff,
							db.GetDefaultRetrySettings(),
						)
					}
				}(b))
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	return g.Wait()
}

// BatchSizeByPlaceholders returns how often the specified number of placeholders fits
// into Options.MaxPlaceholdersPerStatement, but at least 1.
func (db *DB) BatchSizeByPlaceholders(n int) int {
	s := db.Options.MaxPlaceholdersPerStatement / n
	if s > 0 {
		return s
	}

	return 1
}

// YieldAll executes the query with the supplied scope,
// scans each resulting row into an entity returned by the factory function,
// and streams them into a returned channel.
func (db *DB) YieldAll(ctx context.Context, factoryFunc EntityFactoryFunc, query string, scope interface{}) (<-chan Entity, <-chan error) {
	entities := make(chan Entity, 1)
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var counter com.Counter
		defer db.Log(ctx, query, &counter).Stop()
		defer close(entities)

		rows, err := db.NamedQueryContext(ctx, query, scope)
		if err != nil {
			return CantPerformQuery(err, query)
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			e := factoryFunc()

			if err := rows.StructScan(e); err != nil {
				return errors.Wrapf(err, "can't store query result into a %T: %s", e, query)
			}

			select {
			case entities <- e:
				counter.Inc()
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return nil
	})

	return entities, com.WaitAsync(g)
}

// CreateStreamed bulk creates the specified entities via NamedBulkExec.
// The insert statement is created using BuildInsertStmt with the first entity from the entities stream.
// Bulk size is controlled via Options.MaxPlaceholdersPerStatement and
// concurrency is controlled via Options.MaxConnectionsPerTable.
// Entities for which the query ran successfully will be passed to onSuccess.
func (db *DB) CreateStreamed(
	ctx context.Context, entities <-chan Entity, onSuccess ...OnSuccess[Entity],
) error {
	first, forward, err := com.CopyFirst(ctx, entities)
	if err != nil {
		return errors.Wrap(err, "can't copy first entity")
	}

	sem := db.GetSemaphoreForTable(TableName(first))
	stmt, placeholders := db.BuildInsertStmt(first)

	return db.NamedBulkExec(
		ctx, stmt, db.BatchSizeByPlaceholders(placeholders), sem,
		forward, com.NeverSplit[Entity], onSuccess...,
	)
}

// CreateIgnoreStreamed bulk creates the specified entities via NamedBulkExec.
// The insert statement is created using BuildInsertIgnoreStmt with the first entity from the entities stream.
// Bulk size is controlled via Options.MaxPlaceholdersPerStatement and
// concurrency is controlled via Options.MaxConnectionsPerTable.
// Entities for which the query ran successfully will be passed to onSuccess.
func (db *DB) CreateIgnoreStreamed(
	ctx context.Context, entities <-chan Entity, onSuccess ...OnSuccess[Entity],
) error {
	first, forward, err := com.CopyFirst(ctx, entities)
	if err != nil {
		return errors.Wrap(err, "can't copy first entity")
	}

	sem := db.GetSemaphoreForTable(TableName(first))
	stmt, placeholders := db.BuildInsertIgnoreStmt(first)

	return db.NamedBulkExec(
		ctx, stmt, db.BatchSizeByPlaceholders(placeholders), sem,
		forward, SplitOnDupId[Entity], onSuccess...,
	)
}

// UpsertStreamed bulk upserts the specified entities via NamedBulkExec.
// The upsert statement is created using BuildUpsertStmt with the first entity from the entities stream.
// Bulk size is controlled via Options.MaxPlaceholdersPerStatement and
// concurrency is controlled via Options.MaxConnectionsPerTable.
// Entities for which the query ran successfully will be passed to onSuccess.
func (db *DB) UpsertStreamed(
	ctx context.Context, entities <-chan Entity, onSuccess ...OnSuccess[Entity],
) error {
	first, forward, err := com.CopyFirst(ctx, entities)
	if err != nil {
		return errors.Wrap(err, "can't copy first entity")
	}

	sem := db.GetSemaphoreForTable(TableName(first))
	stmt, placeholders := db.BuildUpsertStmt(first)

	return db.NamedBulkExec(
		ctx, stmt, db.BatchSizeByPlaceholders(placeholders), sem,
		forward, SplitOnDupId[Entity], onSuccess...,
	)
}

// UpdateStreamed bulk updates the specified entities via NamedBulkExecTx.
// The update statement is created using BuildUpdateStmt with the first entity from the entities stream.
// Bulk size is controlled via Options.MaxRowsPerTransaction and
// concurrency is controlled via Options.MaxConnectionsPerTable.
func (db *DB) UpdateStreamed(ctx context.Context, entities <-chan Entity) error {
	first, forward, err := com.CopyFirst(ctx, entities)
	if err != nil {
		return errors.Wrap(err, "can't copy first entity")
	}
	sem := db.GetSemaphoreForTable(TableName(first))
	stmt, _ := db.BuildUpdateStmt(first)

	return db.NamedBulkExecTx(ctx, stmt, db.Options.MaxRowsPerTransaction, sem, forward)
}

// DeleteStreamed bulk deletes the specified ids via BulkExec.
// The delete statement is created using BuildDeleteStmt with the passed entityType.
// Bulk size is controlled via Options.MaxPlaceholdersPerStatement and
// concurrency is controlled via Options.MaxConnectionsPerTable.
// IDs for which the query ran successfully will be passed to onSuccess.
func (db *DB) DeleteStreamed(
	ctx context.Context, entityType Entity, ids <-chan interface{}, onSuccess ...OnSuccess[any],
) error {
	sem := db.GetSemaphoreForTable(TableName(entityType))
	return db.BulkExec(
		ctx, db.BuildDeleteStmt(entityType), db.Options.MaxPlaceholdersPerStatement, sem, ids, onSuccess...,
	)
}

// Delete creates a channel from the specified ids and
// bulk deletes them by passing the channel along with the entityType to DeleteStreamed.
// IDs for which the query ran successfully will be passed to onSuccess.
func (db *DB) Delete(
	ctx context.Context, entityType Entity, ids []interface{}, onSuccess ...OnSuccess[any],
) error {
	idsCh := make(chan interface{}, len(ids))
	for _, id := range ids {
		idsCh <- id
	}
	close(idsCh)

	return db.DeleteStreamed(ctx, entityType, idsCh, onSuccess...)
}

// ExecTx executes the provided function within a database transaction.
//
// Starts a new transaction, executes the provided function, and commits the transaction
// if the function succeeds. If the function returns an error, the transaction is rolled back.
//
// Returns an error if starting the transaction, executing the function, or committing the transaction fails.
//
// Note that committing the transaction may not honor the context provided. For some database drivers, once a COMMIT
// query is started, it will block until the database responds. Therefore, for time-critical scenarios, it is
// recommended to add a select wrapper against the context.
func (db *DB) ExecTx(ctx context.Context, fn func(context.Context, *sqlx.Tx) error) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "can't start transaction")
	}
	// We don't expect meaningful errors from rolling back the tx other than the sql.ErrTxDone, so just ignore it.
	defer func() { _ = tx.Rollback() }()

	if err := fn(ctx, tx); err != nil {
		return errors.WithStack(err)
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "can't commit transaction")
	}

	return nil
}

func (db *DB) GetSemaphoreForTable(table string) *semaphore.Weighted {
	db.tableSemaphoresMu.Lock()
	defer db.tableSemaphoresMu.Unlock()

	if sem, ok := db.tableSemaphores[table]; ok {
		return sem
	} else {
		sem = semaphore.NewWeighted(int64(db.Options.MaxConnectionsPerTable))
		db.tableSemaphores[table] = sem
		return sem
	}
}

// HasTable checks whether a table is present in the database.
//
// The first return value indicates whether a table of the given name exists. The second return value contains any
// errors that occurred during the check. If the error is not nil, the first argument is always false.
func (db *DB) HasTable(ctx context.Context, table string) (bool, error) {
	var tableSchemaFunc string
	switch db.DriverName() {
	case MySQL:
		tableSchemaFunc = "DATABASE()"
	case PostgreSQL:
		tableSchemaFunc = "CURRENT_SCHEMA()"
	default:
		return false, errors.Errorf("unsupported database driver %q", db.DriverName())
	}

	var hasTable bool
	err := retry.WithBackoff(
		ctx,
		func(ctx context.Context) error {
			query := db.Rebind("SELECT 1 FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=" + tableSchemaFunc + " AND TABLE_NAME=?")
			rows, err := db.QueryContext(ctx, query, table)
			if err != nil {
				return CantPerformQuery(err, query)
			}
			defer func() { _ = rows.Close() }()
			hasTable = rows.Next()
			if err := rows.Close(); err != nil {
				return err
			}
			return rows.Err()
		},
		retry.Retryable,
		backoff.DefaultBackoff,
		db.GetDefaultRetrySettings())
	if err != nil {
		return false, errors.Wrapf(err, "can't verify existence of database table %q", table)
	}
	return hasTable, nil
}

func (db *DB) GetDefaultRetrySettings() retry.Settings {
	return retry.Settings{
		Timeout: retry.DefaultTimeout,
		OnRetryableError: func(elapsed time.Duration, attempt uint64, err, lastErr error) {
			db.logger.Warnw("Can't execute query. Retrying",
				zap.Error(err),
				zap.Duration("after", elapsed),
				zap.Uint64("attempt", attempt))
		},
		OnSuccess: func(elapsed time.Duration, attempt uint64, lastErr error) {
			if attempt > 1 {
				db.logger.Infow("Query retried successfully after error",
					zap.Duration("after", elapsed),
					zap.Uint64("attempts", attempt),
					zap.NamedError("recovered_error", lastErr))
			}
		},
	}
}

func (db *DB) Log(ctx context.Context, query string, counter *com.Counter) periodic.Stopper {
	return periodic.Start(ctx, db.logger.Interval(), func(tick periodic.Tick) {
		if count := counter.Reset(); count > 0 {
			db.logger.Debugf("Executed %q with %d rows", query, count)
		}
	}, periodic.OnStop(func(tick periodic.Tick) {
		db.logger.Debugf("Finished executing %q with %d rows in %s", query, counter.Total(), tick.Elapsed)
	}))
}

var (
	// Assert TxOrDB interface compliance of the DB and sqlx.Tx types.
	_ TxOrDB = (*DB)(nil)
	_ TxOrDB = (*sqlx.Tx)(nil)
)
