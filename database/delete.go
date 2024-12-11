package database

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"reflect"
	"time"
)

// DeleteStatement is the interface for building DELETE statements.
type DeleteStatement interface {
	// From sets the table name for the DELETE statement.
	// Overrides the table name provided by the entity.
	From(table string) DeleteStatement

	// SetWhere sets the where clause for the DELETE statement.
	SetWhere(where string) DeleteStatement

	// Entity returns the entity associated with the DELETE statement.
	Entity() Entity

	// Table returns the table name for the DELETE statement.
	Table() string

	Where() string
}

// NewDeleteStatement returns a new deleteStatement for the given entity.
func NewDeleteStatement(entity Entity) DeleteStatement {
	return &deleteStatement{
		entity: entity,
	}
}

// deleteStatement is the default implementation of the DeleteStatement interface.
type deleteStatement struct {
	entity Entity
	table  string
	where  string
}

func (d *deleteStatement) From(table string) DeleteStatement {
	d.table = table

	return d
}

func (d *deleteStatement) SetWhere(where string) DeleteStatement {
	d.where = where

	return d
}

func (d *deleteStatement) Entity() Entity {
	return d.entity
}

func (d *deleteStatement) Table() string {
	return d.table
}

func (d *deleteStatement) Where() string {
	return d.where
}

// DeleteOption is a functional option for DeleteStreamed().
type DeleteOption func(opts *deleteOptions)

// WithDeleteStatement sets the DELETE statement to be used for deleting entities.
func WithDeleteStatement(stmt DeleteStatement) DeleteOption {
	return func(opts *deleteOptions) {
		opts.stmt = stmt
	}
}

// WithOnDelete sets the callbacks for a successful DELETE operation.
func WithOnDelete(onDelete ...OnSuccess[any]) DeleteOption {
	return func(opts *deleteOptions) {
		opts.onDelete = onDelete
	}
}

// deleteOptions stores the options for DeleteStreamed.
type deleteOptions struct {
	stmt     DeleteStatement
	onDelete []OnSuccess[any]
}

// DeleteStreamed deletes entities from the given channel from the database.
func DeleteStreamed(
	ctx context.Context,
	db *DB,
	entityType Entity,
	entities <-chan any,
	options ...DeleteOption,
) error {
	opts := &deleteOptions{}
	for _, option := range options {
		option(opts)
	}

	first, forward, err := com.CopyFirst(ctx, entities)
	if err != nil {
		return errors.Wrap(err, "can't copy first entity")
	}

	sem := db.GetSemaphoreForTable(TableName(entityType))

	var stmt string

	if opts.stmt != nil {
		stmt, err = db.QueryBuilder().DeleteStatement(opts.stmt)
		if err != nil {
			return err
		}
	} else {
		stmt, err = db.QueryBuilder().DeleteStatement(NewDeleteStatement(entityType))
		if err != nil {
			return err
		}
	}

	switch reflect.TypeOf(first).Kind() {
	case reflect.Struct, reflect.Map:
		return namedBulkExec(ctx, db, stmt, db.Options.MaxPlaceholdersPerStatement, sem, forward, com.NeverSplit[any], opts.onDelete...)
	default:
		return bulkExec(ctx, db, stmt, db.Options.MaxPlaceholdersPerStatement, sem, forward, opts.onDelete...)
	}
}

func bulkExec(
	ctx context.Context, db *DB, query string, count int, sem *semaphore.Weighted, arg <-chan any, onSuccess ...OnSuccess[any],
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

			g.Go(func(b []any) func() error {
				return func() error {
					defer sem.Release(1)

					return retry.WithBackoff(
						ctx,
						func(context.Context) error {
							var valCollection []any

							for _, v := range b {
								val := reflect.ValueOf(v)
								if val.Kind() == reflect.Slice {
									for i := 0; i < val.Len(); i++ {
										valCollection = append(valCollection, val.Index(i).Interface())
									}
								} else {
									valCollection = append(valCollection, val.Interface())
								}
							}

							stmt, args, err := sqlx.In(query, valCollection)
							if err != nil {
								return fmt.Errorf(
									"%w: %w",
									retry.ErrNotRetryable,
									errors.Wrapf(err, "can't build placeholders for %q", query),
								)
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
						backoff.NewExponentialWithJitter(1*time.Millisecond, 1*time.Second),
						db.GetDefaultRetrySettings(),
					)
				}
			}(b))
		}

		return g.Wait()
	})

	return g.Wait()
}
