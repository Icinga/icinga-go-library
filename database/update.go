package database

import "context"

// UpdateStatement is the interface for building UPDATE statements.
type UpdateStatement interface {
	// SetTable sets the table name for the UPDATE statement.
	// Overrides the table name provided by the entity.
	SetTable(table string) UpdateStatement

	// SetSet sets the set clause for the UPDATE statement.
	SetSet(set string) UpdateStatement

	// SetWhere sets the where clause for the UPDATE statement.
	SetWhere(where string) UpdateStatement

	// Entity returns the entity associated with the UPDATE statement.
	Entity() Entity

	// Table returns the table name for the UPDATE statement.
	Table() string

	// Set returns the set clause for the UPDATE statement.
	Set() string

	// Where returns the where clause for the UPDATE statement.
	Where() string

	// apply implements the UpdateOption interface and applies itself to the given options.
	apply(opts *updateOptions)
}

// NewUpdateStatement returns a new updateStatement for the given entity.
func NewUpdateStatement(entity Entity) UpdateStatement {
	return &updateStatement{
		entity: entity,
	}
}

// updateStatement is the default implementation of the UpdateStatement interface.
type updateStatement struct {
	entity Entity
	table  string
	set    string
	where  string
}

func (u *updateStatement) SetTable(table string) UpdateStatement {
	u.table = table

	return u
}

func (u *updateStatement) SetSet(set string) UpdateStatement {
	u.set = set

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

func (u *updateStatement) Set() string {
	return u.set
}

func (u *updateStatement) Where() string {
	return u.where
}

func (u *updateStatement) apply(opts *updateOptions) {
	opts.stmt = u
}

// UpdateOption is the interface for functional options for UpdateStatement.
type UpdateOption interface {
	// apply applies the option to the given updateOptions.
	apply(opts *updateOptions)
}

// UpdateOptionFunc is a function type that implements the UpdateOption interface.
type UpdateOptionFunc func(opts *updateOptions)

func (f UpdateOptionFunc) apply(opts *updateOptions) {
	f(opts)
}

// WithOnUpdate sets the callback functions to be called after a successful UPDATE.
func WithOnUpdate(onUpdate ...OnSuccess[any]) UpdateOption {
	return UpdateOptionFunc(func(opts *updateOptions) {
		opts.onUpdate = onUpdate
	})
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
