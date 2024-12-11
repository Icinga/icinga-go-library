package database

import "context"

// UpdateStatement is the interface for building UPDATE statements.
type UpdateStatement interface {
	// SetTable sets the table name for the UPDATE statement.
	// Overrides the table name provided by the entity.
	SetTable(table string) UpdateStatement

	// SetColumns sets the columns to be updated.
	SetColumns(columns ...string) UpdateStatement

	// SetExcludedColumns sets the columns to be excluded from the UPDATE statement.
	// Excludes also columns set by SetColumns.
	SetExcludedColumns(columns ...string) UpdateStatement

	// SetWhere sets the where clause for the UPDATE statement.
	SetWhere(where string) UpdateStatement

	// Entity returns the entity associated with the UPDATE statement.
	Entity() Entity

	// Table returns the table name for the UPDATE statement.
	Table() string

	// Columns returns the columns to be updated.
	Columns() []string

	// ExcludedColumns returns the columns to be excluded from the UPDATE statement.
	ExcludedColumns() []string

	// Where returns the where clause for the UPDATE statement.
	Where() string
}

// NewUpdateStatement returns a new updateStatement for the given entity.
func NewUpdateStatement(entity Entity) UpdateStatement {
	return &updateStatement{
		entity: entity,
	}
}

// updateStatement is the default implementation of the UpdateStatement interface.
type updateStatement struct {
	entity          Entity
	table           string
	columns         []string
	excludedColumns []string
	where           string
}

func (u *updateStatement) SetTable(table string) UpdateStatement {
	u.table = table

	return u
}

func (u *updateStatement) SetColumns(columns ...string) UpdateStatement {
	u.columns = columns

	return u
}

func (u *updateStatement) SetExcludedColumns(columns ...string) UpdateStatement {
	u.excludedColumns = columns

	return u
}

func (u *updateStatement) SetWhere(where string) UpdateStatement {
	u.where = where

	return u
}

func (u *updateStatement) Entity() Entity {
	return u.entity
}

func (u *updateStatement) Table() string {
	return u.table
}

func (u *updateStatement) Columns() []string {
	return u.columns
}

func (u *updateStatement) ExcludedColumns() []string {
	return u.excludedColumns
}

func (u *updateStatement) Where() string {
	return u.where
}

// UpdateOption is a functional option for UpdateStreamed().
type UpdateOption func(opts *updateOptions)

// WithUpdateStatement sets the UPDATE statement to be used for updating entities.
func WithUpdateStatement(stmt UpdateStatement) UpdateOption {
	return func(opts *updateOptions) {
		opts.stmt = stmt
	}
}

// WithOnUpdate sets the callback functions to be called after a successful UPDATE.
func WithOnUpdate(onUpdate ...OnSuccess[any]) UpdateOption {
	return func(opts *updateOptions) {
		opts.onUpdate = append(opts.onUpdate, onUpdate...)
	}
}

// updateOptions stores the options for UpdateStreamed.
type updateOptions struct {
	stmt     UpdateStatement
	onUpdate []OnSuccess[any]
}

func UpdateStreamed[T any, V EntityConstraint[T]](
	ctx context.Context,
	db *DB,
	entities <-chan T,
	options ...UpdateOption,
) error {
	// TODO (jr): implement
	return nil
}
