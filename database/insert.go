package database

import "context"

// InsertStatement is the interface for building INSERT statements.
type InsertStatement interface {
	// Into sets the table name for the INSERT statement.
	// Overrides the table name provided by the entity.
	Into(table string) InsertStatement

	// SetColumns sets the columns to be inserted.
	SetColumns(columns ...string) InsertStatement

	// SetExcludedColumns sets the columns to be excluded from the INSERT statement.
	// Excludes also columns set by SetColumns.
	SetExcludedColumns(columns ...string) InsertStatement

	// Entity returns the entity associated with the INSERT statement.
	Entity() Entity

	// Table returns the table name for the INSERT statement.
	Table() string

	// Columns returns the columns to be inserted.
	Columns() []string

	// ExcludedColumns returns the columns to be excluded from the INSERT statement.
	ExcludedColumns() []string

	// apply implements the InsertOption interface and applies itself to the given options.
	apply(opts *insertOptions)
}

// NewInsertStatement returns a new insertStatement for the given entity.
func NewInsertStatement(entity Entity) InsertStatement {
	return &insertStatement{
		entity: entity,
	}
}

// insertStatement is the default implementation of the InsertStatement interface.
type insertStatement struct {
	entity          Entity
	table           string
	columns         []string
	excludedColumns []string
}

func (i *insertStatement) Into(table string) InsertStatement {
	i.table = table

	return i
}

func (i *insertStatement) SetColumns(columns ...string) InsertStatement {
	i.columns = columns

	return i
}

func (i *insertStatement) SetExcludedColumns(columns ...string) InsertStatement {
	i.excludedColumns = columns

	return i
}

func (i *insertStatement) Entity() Entity {
	return i.entity
}

func (i *insertStatement) Table() string {
	return i.table
}

func (i *insertStatement) Columns() []string {
	return i.columns
}

func (i *insertStatement) ExcludedColumns() []string {
	return i.excludedColumns
}

func (i *insertStatement) apply(opts *insertOptions) {
	opts.stmt = i
}

// InsertSelectStatement is the interface for building INSERT SELECT statements.
type InsertSelectStatement interface {
	InsertStatement

	// SetSelect sets the SELECT statement for the INSERT SELECT statement.
	SetSelect(stmt SelectStatement) InsertSelectStatement

	// Select returns the SELECT statement for the INSERT SELECT statement.
	Select() SelectStatement
}

// NewInsertSelect returns a new insertSelectStatement for the given entity.
func NewInsertSelect(entity Entity) InsertSelectStatement {
	return &insertSelectStatement{
		insertStatement: insertStatement{
			entity: entity,
		},
	}
}

// insertSelectStatement is the default implementation of the InsertSelectStatement interface.
type insertSelectStatement struct {
	insertStatement
	selectStmt SelectStatement
}

func (i *insertSelectStatement) SetSelect(stmt SelectStatement) InsertSelectStatement {
	i.selectStmt = stmt

	return i
}

func (i *insertSelectStatement) Select() SelectStatement {
	return i.selectStmt
}

// InsertOption is the interface for functional options for InsertStreamed.
type InsertOption interface {
	// apply applies the option to the given insertOptions.
	apply(opts *insertOptions)
}

// InsertOptionFunc is a function type that implements the InsertOption interface.
type InsertOptionFunc func(opts *insertOptions)

func (f InsertOptionFunc) apply(opts *insertOptions) {
	f(opts)
}

// WithOnInsert sets the onInsert callbacks for a successful INSERT statement.
func WithOnInsert(onInsert ...OnSuccess[any]) InsertOption {
	return InsertOptionFunc(func(opts *insertOptions) {
		opts.onInsert = onInsert
	})
}

// insertOptions stores the options for InsertStreamed.
type insertOptions struct {
	stmt     InsertStatement
	onInsert []OnSuccess[any]
}

// InsertStreamed inserts entities from the given channel into the database.
func InsertStreamed[T any, V EntityConstraint[T]](
	ctx context.Context,
	db *DB,
	entities <-chan T,
	options ...InsertOption,
) error {
	// TODO (jr): implement
	return nil
}
