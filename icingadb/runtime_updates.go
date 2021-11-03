package icingadb

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/contracts"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/periodic"
	"github.com/icinga/icingadb/pkg/structify"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"reflect"
	"sync"
)

// RuntimeUpdates specifies the source and destination of runtime updates.
type RuntimeUpdates struct {
	db     *DB
	redis  *icingaredis.Client
	logger *zap.SugaredLogger
}

// NewRuntimeUpdates creates a new RuntimeUpdates.
func NewRuntimeUpdates(db *DB, redis *icingaredis.Client, logger *zap.SugaredLogger) *RuntimeUpdates {
	return &RuntimeUpdates{
		db:     db,
		redis:  redis,
		logger: logger,
	}
}

const bulkSize = 1 << 14

// Streams returns the stream key to ID mapping of the runtime update streams for later use in Sync.
func (r *RuntimeUpdates) Streams(ctx context.Context) (config, state icingaredis.Streams, err error) {
	config = icingaredis.Streams{"icinga:runtime": "0-0"}
	state = icingaredis.Streams{"icinga:runtime:state": "0-0"}

	for _, streams := range [...]icingaredis.Streams{config, state} {
		for key := range streams {
			id, err := r.redis.StreamLastId(ctx, key)
			if err != nil {
				return nil, nil, err
			}

			streams[key] = id
		}
	}

	return
}

// Sync synchronizes runtime update streams from s.redis to s.db and deletes the original data on success.
// Note that Sync must be only be called configuration synchronization has been completed.
func (r *RuntimeUpdates) Sync(ctx context.Context, factoryFuncs []contracts.EntityFactoryFunc, streams icingaredis.Streams) error {
	g, ctx := errgroup.WithContext(ctx)

	updateMessagesByKey := make(map[string]chan<- redis.XMessage)

	for _, factoryFunc := range factoryFuncs {
		s := common.NewSyncSubject(factoryFunc)

		updateMessages := make(chan redis.XMessage, bulkSize)
		upsertEntities := make(chan contracts.Entity, bulkSize)
		deleteIds := make(chan interface{}, bulkSize)

		updateMessagesByKey[fmt.Sprintf("icinga:%s", utils.Key(s.Name(), ':'))] = updateMessages

		r.logger.Debugf("Syncing runtime updates of %s", s.Name())
		g.Go(structifyStream(ctx, updateMessages, upsertEntities, deleteIds, structify.MakeMapStructifier(reflect.TypeOf(s.Entity()).Elem(), "json")))

		upserted := make(chan contracts.Entity)
		g.Go(func() error {
			defer close(upserted)

			stmt, placeholders := r.db.BuildUpsertStmt(s.Entity())
			// Updates must be executed in order, ensure this by using a semaphore with maximum 1.
			sem := semaphore.NewWeighted(1)
			return r.db.NamedBulkExec(ctx, stmt, r.db.BatchSizeByPlaceholders(placeholders), sem, upsertEntities, upserted)
		})
		g.Go(func() error {
			var counter com.Counter
			defer periodic.Start(ctx, internal.LoggingInterval(), func(_ periodic.Tick) {
				if count := counter.Reset(); count > 0 {
					r.logger.Infof("Upserted %d %s items", count, s.Name())
				}
			}).Stop()

			for {
				select {
				case _, ok := <-upserted:
					if !ok {
						return nil
					}

					counter.Inc()
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		deleted := make(chan interface{})
		g.Go(func() error {
			defer close(deleted)

			return r.db.DeleteStreamed(ctx, s.Entity(), deleteIds, deleted)
		})
		g.Go(func() error {
			var counter com.Counter
			defer periodic.Start(ctx, internal.LoggingInterval(), func(_ periodic.Tick) {
				if count := counter.Reset(); count > 0 {
					r.logger.Infof("Deleted %d %s items", count, s.Name())
				}
			}).Stop()

			for {
				select {
				case _, ok := <-deleted:
					if !ok {
						return nil
					}

					counter.Inc()
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})
	}

	// customvar and customvar_flat sync.
	{
		updateMessages := make(chan redis.XMessage, bulkSize)
		upsertEntities := make(chan contracts.Entity, bulkSize)
		deleteIds := make(chan interface{}, bulkSize)

		cv := common.NewSyncSubject(v1.NewCustomvar)
		cvFlat := common.NewSyncSubject(v1.NewCustomvarFlat)

		r.logger.Debug("Syncing runtime updates of " + cv.Name())
		r.logger.Debug("Syncing runtime updates of " + cvFlat.Name())

		updateMessagesByKey["icinga:"+utils.Key(cv.Name(), ':')] = updateMessages
		g.Go(structifyStream(ctx, updateMessages, upsertEntities, deleteIds, structify.MakeMapStructifier(reflect.TypeOf(cv.Entity()).Elem(), "json")))

		customvars, flatCustomvars, errs := v1.ExpandCustomvars(ctx, upsertEntities)
		com.ErrgroupReceive(g, errs)

		upsertedCustomvars := make(chan contracts.Entity)
		g.Go(func() error {
			defer close(upsertedCustomvars)

			stmt, placeholders := r.db.BuildUpsertStmt(cv.Entity())
			// Updates must be executed in order, ensure this by using a semaphore with maximum 1.
			sem := semaphore.NewWeighted(1)
			return r.db.NamedBulkExec(ctx, stmt, r.db.BatchSizeByPlaceholders(placeholders), sem, customvars, upsertedCustomvars)
		})
		g.Go(func() error {
			var counter com.Counter
			defer periodic.Start(ctx, internal.LoggingInterval(), func(_ periodic.Tick) {
				if count := counter.Reset(); count > 0 {
					r.logger.Infof("Upserted %d %s items", count, cv.Name())
				}
			}).Stop()

			for {
				select {
				case _, ok := <-upsertedCustomvars:
					if !ok {
						return nil
					}

					counter.Inc()
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		upsertedFlatCustomvars := make(chan contracts.Entity)
		g.Go(func() error {
			defer close(upsertedFlatCustomvars)

			stmt, placeholders := r.db.BuildUpsertStmt(cvFlat.Entity())
			// Updates must be executed in order, ensure this by using a semaphore with maximum 1.
			sem := semaphore.NewWeighted(1)
			return r.db.NamedBulkExec(ctx, stmt, r.db.BatchSizeByPlaceholders(placeholders), sem, flatCustomvars, upsertedFlatCustomvars)
		})
		g.Go(func() error {
			var counter com.Counter
			defer periodic.Start(ctx, internal.LoggingInterval(), func(_ periodic.Tick) {
				if count := counter.Reset(); count > 0 {
					r.logger.Infof("Upserted %d %s items", count, cvFlat.Name())
				}
			}).Stop()

			for {
				select {
				case _, ok := <-upsertedFlatCustomvars:
					if !ok {
						return nil
					}

					counter.Inc()
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		g.Go(func() error {
			var once sync.Once
			for {
				select {
				case _, ok := <-deleteIds:
					if !ok {
						return nil
					}
					// Icinga 2 does not send custom var delete events.
					once.Do(func() {
						r.logger.DPanic("received unexpected custom var delete event")
					})
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})
	}

	g.Go(r.xRead(ctx, updateMessagesByKey, streams))

	return g.Wait()
}

// xRead reads from the runtime update streams and sends the data to the corresponding updateMessages channel.
// The updateMessages channel is determined by a "redis_key" on each redis message.
func (r *RuntimeUpdates) xRead(ctx context.Context, updateMessagesByKey map[string]chan<- redis.XMessage, streams icingaredis.Streams) func() error {
	return func() error {
		defer func() {
			for _, updateMessages := range updateMessagesByKey {
				close(updateMessages)
			}
		}()

		for {
			xra := &redis.XReadArgs{
				Streams: streams.Option(),
				Count:   bulkSize,
				Block:   0,
			}

			cmd := r.redis.XRead(ctx, xra)
			rs, err := cmd.Result()

			if err != nil {
				return icingaredis.WrapCmdErr(cmd)
			}

			for _, stream := range rs {
				var id string

				for _, message := range stream.Messages {
					id = message.ID

					redisKey := message.Values["redis_key"]
					if redisKey == nil {
						return errors.Errorf("stream message missing 'redis_key' key: %v", message.Values)
					}

					updateMessages := updateMessagesByKey[redisKey.(string)]
					if updateMessages == nil {
						return errors.Errorf("no object type for redis key %s found", redisKey)
					}

					select {
					case updateMessages <- message:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				streams[stream.Stream] = id
			}
		}
	}
}

// structifyStream gets Redis stream messages (redis.XMessage) via the updateMessages channel and converts
// those messages into Icinga DB entities (contracts.Entity) using the provided structifier.
// Converted entities are inserted into the upsertEntities or deleteIds channel depending on the "runtime_type" message field.
func structifyStream(ctx context.Context, updateMessages <-chan redis.XMessage, upsertEntities chan contracts.Entity, deleteIds chan interface{}, structifier structify.MapStructifier) func() error {
	return func() error {
		defer func() {
			close(upsertEntities)
			close(deleteIds)
		}()

		for {
			select {
			case message, ok := <-updateMessages:
				if !ok {
					return nil
				}

				ptr, err := structifier(message.Values)
				if err != nil {
					return errors.Wrapf(err, "can't structify values %#v", message.Values)
				}

				entity := ptr.(contracts.Entity)

				runtimeType := message.Values["runtime_type"]
				if runtimeType == nil {
					return errors.Errorf("stream message missing 'runtime_type' key: %v", message.Values)
				}

				if runtimeType == "upsert" {
					select {
					case upsertEntities <- entity:
					case <-ctx.Done():
						return ctx.Err()
					}
				} else if runtimeType == "delete" {
					select {
					case deleteIds <- entity.ID():
					case <-ctx.Done():
						return ctx.Err()
					}
				} else {
					return errors.Errorf("invalid runtime type: %s", runtimeType)
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
