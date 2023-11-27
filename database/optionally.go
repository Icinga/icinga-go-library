package database

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/com"
	"github.com/pkg/errors"
)

// Upsert inserts new rows into a table or updates rows of a table if the primary key already exists.
type Upsert interface {
	// Stream bulk upserts the specified entities via NamedBulkExec.
	// If not explicitly specified, the upsert statement is created using
	// BuildUpsertStmt with the first entity from the entities stream.
	Stream(ctx context.Context, entities <-chan Entity) error
}

// UpsertOption is a functional option for NewUpsert.
type UpsertOption func(u *upsert)

// WithOnUpsert adds callback(s) to bulk upserts. Entities for which the
// operation was performed successfully are passed to the callbacks.
func WithOnUpsert(onUpsert ...OnSuccess[Entity]) UpsertOption {
	return func(u *upsert) {
		u.onUpsert = onUpsert
	}
}

// WithStatement uses the specified statement for bulk upserts instead of automatically creating one.
func WithStatement(stmt string, placeholders int) UpsertOption {
	return func(u *upsert) {
		u.stmt = stmt
		u.placeholders = placeholders
	}
}

// NewUpsert creates a new Upsert initialized with a database.
func NewUpsert(db *DB, options ...UpsertOption) Upsert {
	u := &upsert{db: db}

	for _, option := range options {
		option(u)
	}

	return u
}

type upsert struct {
	db           *DB
	onUpsert     []OnSuccess[Entity]
	stmt         string
	placeholders int
}

func (u *upsert) Stream(ctx context.Context, entities <-chan Entity) error {
	first, forward, err := com.CopyFirst(ctx, entities)
	if err != nil {
		return errors.Wrap(err, "can't copy first entity")
	}

	sem := u.db.GetSemaphoreForTable(TableName(first))
	var stmt string
	var placeholders int

	if u.stmt != "" {
		stmt = u.stmt
		placeholders = u.placeholders
	} else {
		stmt, placeholders = u.db.BuildUpsertStmt(first)
	}

	return u.db.NamedBulkExec(
		ctx, stmt, u.db.BatchSizeByPlaceholders(placeholders), sem,
		forward, SplitOnDupId[Entity], u.onUpsert...,
	)
}

// Delete deletes rows of a table.
type Delete interface {
	// Stream bulk deletes rows from the table specified in from using the given args stream via BulkExec.
	// Unless explicitly specified, the DELETE statement is created using BuildDeleteStmt.
	Stream(ctx context.Context, from any, args <-chan any) error
}

// DeleteOption is a functional option for NewDelete.
type DeleteOption func(options *delete)

// WithOnDelete adds callback(s) to bulk deletes. Arguments for which the
// operation was performed successfully are passed to the callbacks.
func WithOnDelete(onDelete ...OnSuccess[any]) DeleteOption {
	return func(d *delete) {
		d.onDelete = onDelete
	}
}

// ByColumn uses the given column for the WHERE clause that the rows must
// satisfy in order to be deleted, instead of automatically using ID.
func ByColumn(column string) DeleteOption {
	return func(d *delete) {
		d.column = column
	}
}

// NewDelete creates a new Delete initialized with a database.
func NewDelete(db *DB, options ...DeleteOption) Delete {
	d := &delete{db: db}

	for _, option := range options {
		option(d)
	}

	return d
}

type delete struct {
	db       *DB
	column   string
	onDelete []OnSuccess[any]
}

func (d *delete) Stream(ctx context.Context, from any, args <-chan any) error {
	var stmt string

	if d.column != "" {
		stmt = fmt.Sprintf(`DELETE FROM "%s" WHERE %s IN (?)`, TableName(from), d.column)
	} else {
		stmt = d.db.BuildDeleteStmt(from)
	}

	sem := d.db.GetSemaphoreForTable(TableName(from))

	return d.db.BulkExec(
		ctx, stmt, d.db.Options.MaxPlaceholdersPerStatement, sem, args, d.onDelete...,
	)
}
