package database

import (
	"context"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"time"
)

type UpsertStatement interface {
	Into(table string) UpsertStatement

	SetColumns(columns ...string) UpsertStatement

	SetExcludedColumns(columns ...string) UpsertStatement

	Entity() Entity

	Table() string

	Columns() []string

	ExcludedColumns() []string

	apply(opts *upsertOptions)
}

func NewUpsertStatement(entity Entity) UpsertStatement {
	return &upsertStatement{
		entity: entity,
	}
}

type upsertStatement struct {
	entity          Entity
	table           string
	columns         []string
	excludedColumns []string
}

func (u *upsertStatement) Into(table string) UpsertStatement {
	u.table = table

	return u
}

func (u *upsertStatement) SetColumns(columns ...string) UpsertStatement {
	u.columns = columns

	return u
}

func (u *upsertStatement) SetExcludedColumns(columns ...string) UpsertStatement {
	u.excludedColumns = columns

	return u
}

func (u *upsertStatement) Entity() Entity {
	return u.entity
}

func (u *upsertStatement) Table() string {
	return u.table
}

func (u *upsertStatement) Columns() []string {
	return u.columns
}

func (u *upsertStatement) ExcludedColumns() []string {
	return u.excludedColumns
}

func (u *upsertStatement) apply(opts *upsertOptions) {
	opts.stmt = u
}

type UpsertOption interface {
	apply(opts *upsertOptions)
}

type UpsertOptionFunc func(opts *upsertOptions)

func (f UpsertOptionFunc) apply(opts *upsertOptions) {
	f(opts)
}

func WithOnUpsert(onUpsert ...OnSuccess[any]) UpsertOption {
	return UpsertOptionFunc(func(opts *upsertOptions) {
		opts.onUpsert = onUpsert
	})
}

type upsertOptions struct {
	stmt     UpsertStatement
	onUpsert []OnSuccess[any]
}

func UpsertStreamed[T any, V EntityConstraint[T]](
	ctx context.Context,
	db *DB,
	entities <-chan T,
	options ...UpsertOption,
) error {
	opts := &upsertOptions{}
	for _, option := range options {
		option.apply(opts)
	}

	entityType := V(new(T))
	sem := db.GetSemaphoreForTable(TableName(entityType))

	var stmt string
	var placeholders int

	if opts.stmt != nil {
		stmt, placeholders, _ = BuildUpsertStatement(db, opts.stmt)
	} else {
		stmt, placeholders = db.BuildUpsertStmt(entityType)
	}

	return namedBulkExec[T](
		ctx, db, stmt, db.BatchSizeByPlaceholders(placeholders), sem,
		entities, splitOnDupId[T], opts.onUpsert...,
	)
}

func namedBulkExec[T any](
	ctx context.Context,
	db *DB,
	query string,
	count int,
	sem *semaphore.Weighted,
	arg <-chan T,
	splitPolicyFactory com.BulkChunkSplitPolicyFactory[T],
	onSuccess ...OnSuccess[any],
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

				g.Go(func(b []T) func() error {
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
									// TODO (jr): remove -> workaround vvvv
									var items []any
									for _, item := range b {
										items = append(items, any(item))
									}
									// TODO ---- workaround end ---- ^^^^

									if err := onSuccess(ctx, items); err != nil {
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
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	return g.Wait()
}

func splitOnDupId[T any]() com.BulkChunkSplitPolicy[T] {
	seenIds := map[string]struct{}{}

	return func(ider T) bool {
		entity, ok := any(ider).(IDer)
		if !ok {
			panic("Type T does not implement IDer")
		}

		id := entity.ID().String()

		_, ok = seenIds[id]
		if ok {
			seenIds = map[string]struct{}{id: {}}
		} else {
			seenIds[id] = struct{}{}
		}

		return ok
	}
}