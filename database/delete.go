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

type DeleteStatement interface {
	From(table string) DeleteStatement

	SetWhere(where string) DeleteStatement

	Entity() Entity

	Table() string

	Where() string

	apply(do *deleteOptions)
}

func NewDeleteStatement(entity Entity) DeleteStatement {
	return &deleteStatement{
		entity: entity,
	}
}

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

func (d *deleteStatement) apply(opts *deleteOptions) {
	opts.stmt = d
}

type DeleteOption interface {
	apply(*deleteOptions)
}

type DeleteOptionFunc func(opts *deleteOptions)

func (f DeleteOptionFunc) apply(opts *deleteOptions) {
	f(opts)
}

func WithOnDelete(onDelete ...OnSuccess[any]) DeleteOption {
	return DeleteOptionFunc(func(opts *deleteOptions) {
		opts.onDelete = onDelete
	})
}

type deleteOptions struct {
	stmt     DeleteStatement
	onDelete []OnSuccess[any]
}

func DeleteStreamed(
	ctx context.Context,
	db *DB,
	entityType Entity,
	entities <-chan any,
	options ...DeleteOption,
) error {
	opts := &deleteOptions{}
	for _, option := range options {
		option.apply(opts)
	}

	first, forward, err := com.CopyFirst(ctx, entities)
	if err != nil {
		return errors.Wrap(err, "can't copy first entity")
	}

	sem := db.GetSemaphoreForTable(TableName(entityType))

	var stmt string

	if opts.stmt != nil {
		stmt, _ = BuildDeleteStatement(db, opts.stmt)
	} else {
		stmt = db.BuildDeleteStmt(entityType)
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
