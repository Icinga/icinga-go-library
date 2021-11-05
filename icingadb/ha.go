package icingadb

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"github.com/google/uuid"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/backoff"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingaredis"
	icingaredisv1 "github.com/icinga/icingadb/pkg/icingaredis/v1"
	"github.com/icinga/icingadb/pkg/types"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"sync"
	"time"
)

var timeout = 60 * time.Second

// HA provides high availability and indicates whether a Takeover or Handover must be made.
type HA struct {
	ctx           context.Context
	cancelCtx     context.CancelFunc
	instanceId    types.Binary
	db            *DB
	environmentMu sync.Mutex
	environment   *v1.Environment
	heartbeat     *icingaredis.Heartbeat
	logger        *zap.SugaredLogger
	responsible   bool
	handover      chan struct{}
	takeover      chan struct{}
	done          chan struct{}
	errOnce       sync.Once
	errMu         sync.Mutex
	err           error
}

// NewHA returns a new HA and starts the controller loop.
func NewHA(ctx context.Context, db *DB, heartbeat *icingaredis.Heartbeat, logger *zap.SugaredLogger) *HA {
	ctx, cancelCtx := context.WithCancel(ctx)

	instanceId := uuid.New()

	ha := &HA{
		ctx:        ctx,
		cancelCtx:  cancelCtx,
		instanceId: instanceId[:],
		db:         db,
		heartbeat:  heartbeat,
		logger:     logger,
		handover:   make(chan struct{}),
		takeover:   make(chan struct{}),
		done:       make(chan struct{}),
	}

	go ha.controller()

	return ha
}

// Close shuts h down.
func (h *HA) Close(ctx context.Context) error {
	// Cancel ctx.
	h.cancelCtx()
	// Wait until the controller loop ended.
	<-h.Done()
	// Remove our instance from the database.
	h.removeInstance(ctx)
	// And return an error, if any.
	return h.Err()
}

// Done returns a channel that's closed when the HA controller loop ended.
func (h *HA) Done() <-chan struct{} {
	return h.done
}

// Environment returns the current environment.
func (h *HA) Environment() *v1.Environment {
	h.environmentMu.Lock()
	defer h.environmentMu.Unlock()

	return h.environment
}

// Err returns an error if Done has been closed and there is an error. Otherwise returns nil.
func (h *HA) Err() error {
	h.errMu.Lock()
	defer h.errMu.Unlock()

	return h.err
}

// Handover returns a channel with which handovers are signaled.
func (h *HA) Handover() chan struct{} {
	return h.handover
}

// Takeover returns a channel with which takeovers are signaled.
func (h *HA) Takeover() chan struct{} {
	return h.takeover
}

func (h *HA) abort(err error) {
	h.errOnce.Do(func() {
		h.errMu.Lock()
		h.err = errors.Wrap(err, "HA aborted")
		h.errMu.Unlock()

		h.cancelCtx()
	})
}

// controller loop.
func (h *HA) controller() {
	defer close(h.done)

	h.logger.Debugw("Starting HA", zap.String("instance_id", hex.EncodeToString(h.instanceId)))

	oldInstancesRemoved := false

	logTicker := time.NewTicker(time.Second * 60)
	defer logTicker.Stop()
	shouldLog := true

	for {
		select {
		case m := <-h.heartbeat.Events():
			if m != nil {
				now := time.Now()
				t, err := m.Stats().Time()
				if err != nil {
					h.abort(err)
				}
				tt := t.Time()
				if tt.After(now.Add(1 * time.Second)) {
					h.logger.Debugw("Received heartbeat from the future", zap.Time("time", tt))
				}
				if tt.Before(now.Add(-1 * timeout)) {
					h.logger.Errorw("Received heartbeat from the past", zap.Time("time", tt))
					h.signalHandover()
					continue
				}
				s, err := m.Stats().IcingaStatus()
				if err != nil {
					h.abort(err)
				}

				envId, err := m.EnvironmentID()
				if err != nil {
					h.abort(err)
				}

				if h.environment == nil || !bytes.Equal(h.environment.Id, envId) {
					if h.environment != nil {
						h.logger.Fatalw("Environment changed unexpectedly",
							zap.String("current", h.environment.Id.String()),
							zap.String("new", envId.String()))
					}

					h.environmentMu.Lock()
					h.environment = &v1.Environment{
						EntityWithoutChecksum: v1.EntityWithoutChecksum{IdMeta: v1.IdMeta{
							Id: envId,
						}},
						Name: types.String{
							NullString: sql.NullString{
								String: envId.String(),
								Valid:  true,
							},
						},
					}
					h.environmentMu.Unlock()
				}

				select {
				case <-logTicker.C:
					shouldLog = true
				default:
				}

				var realizeCtx context.Context
				var cancelRealizeCtx context.CancelFunc
				if h.responsible {
					realizeCtx, cancelRealizeCtx = context.WithDeadline(h.ctx, m.ExpiryTime())
				} else {
					realizeCtx, cancelRealizeCtx = context.WithCancel(h.ctx)
				}
				err = h.realize(realizeCtx, s, t, envId, shouldLog)
				cancelRealizeCtx()
				if errors.Is(err, context.DeadlineExceeded) {
					h.signalHandover()
					continue
				}
				if err != nil {
					h.abort(err)
				}

				if !oldInstancesRemoved {
					go h.removeOldInstances(s, envId)
					oldInstancesRemoved = true
				}

				shouldLog = false
			} else {
				h.logger.Error("Lost heartbeat")
				h.signalHandover()
			}
		case <-h.heartbeat.Done():
			if err := h.heartbeat.Err(); err != nil {
				h.abort(err)
			}
		case <-h.ctx.Done():
			return
		}
	}
}

func (h *HA) realize(ctx context.Context, s *icingaredisv1.IcingaStatus, t *types.UnixMilli, envId types.Binary, shouldLog bool) error {
	boff := backoff.NewExponentialWithJitter(time.Millisecond*256, time.Second*3)
	for attempt := 0; true; attempt++ {
		sleep := boff(uint64(attempt))
		time.Sleep(sleep)

		ctx, cancelCtx := context.WithCancel(ctx)
		tx, err := h.db.BeginTxx(ctx, &sql.TxOptions{
			Isolation: sql.LevelSerializable,
		})
		if err != nil {
			cancelCtx()
			return errors.Wrap(err, "can't start transaction")
		}
		query := `SELECT id, heartbeat FROM icingadb_instance WHERE environment_id = ? AND responsible = ? AND id != ? AND heartbeat > ?`
		rows, err := tx.QueryxContext(ctx, query, envId, "y", h.instanceId, utils.UnixMilli(time.Now().Add(-1*timeout)))
		if err != nil {
			cancelCtx()
			return internal.CantPerformQuery(err, query)
		}
		takeover := true
		if rows.Next() {
			instance := &v1.IcingadbInstance{}
			err := rows.StructScan(instance)
			if err != nil {
				h.logger.Errorw("Can't scan currently active instance", zap.Error(err))
			} else {
				if shouldLog {
					h.logger.Infow("Another instance is active",
						zap.String("instance_id", instance.Id.String()),
						zap.String("environment", envId.String()),
						"heartbeat", instance.Heartbeat,
						zap.Duration("heartbeat_age", time.Since(instance.Heartbeat.Time())))
				}
				takeover = false
			}
		}
		_ = rows.Close()
		i := v1.IcingadbInstance{
			EntityWithoutChecksum: v1.EntityWithoutChecksum{
				IdMeta: v1.IdMeta{
					Id: h.instanceId,
				},
			},
			EnvironmentMeta: v1.EnvironmentMeta{
				EnvironmentId: envId,
			},
			Heartbeat:                         *t,
			Responsible:                       types.Bool{Bool: takeover || h.responsible, Valid: true},
			EndpointId:                        s.EndpointId,
			Icinga2Version:                    s.Version,
			Icinga2StartTime:                  s.ProgramStart,
			Icinga2NotificationsEnabled:       s.NotificationsEnabled,
			Icinga2ActiveServiceChecksEnabled: s.ActiveServiceChecksEnabled,
			Icinga2ActiveHostChecksEnabled:    s.ActiveHostChecksEnabled,
			Icinga2EventHandlersEnabled:       s.EventHandlersEnabled,
			Icinga2FlapDetectionEnabled:       s.FlapDetectionEnabled,
			Icinga2PerformanceDataEnabled:     s.PerformanceDataEnabled,
		}

		stmt, _ := h.db.BuildUpsertStmt(i)
		_, err = tx.NamedExecContext(ctx, stmt, i)

		if err != nil {
			cancelCtx()
			err = internal.CantPerformQuery(err, stmt)
			if !utils.IsDeadlock(err) {
				h.logger.Errorw("Can't update or insert instance", zap.Error(err))
				break
			} else {
				if attempt > 2 {
					// Log with info level after third attempt
					h.logger.Infow("Can't update or insert instance. Retrying", zap.Error(err), zap.Int("retry count", attempt))
				} else {
					h.logger.Debugw("Can't update or insert instance. Retrying", zap.Error(err), zap.Int("retry count", attempt))
				}
				continue
			}
		}

		if err := tx.Commit(); err != nil {
			cancelCtx()
			return errors.Wrap(err, "can't commit transaction")
		}

		if takeover {
			// Insert the environment after each heartbeat takeover if it does not already exist in the database
			// as the environment may have changed, although this is likely to happen very rarely.
			if err := h.insertEnvironment(); err != nil {
				cancelCtx()
				return errors.Wrap(err, "can't insert environment")
			}

			h.signalTakeover()
		}

		cancelCtx()
		break
	}

	return nil
}

// insertEnvironment inserts the environment from the specified state into the database if it does not already exist.
func (h *HA) insertEnvironment() error {
	// Instead of checking whether the environment already exists, use an INSERT statement that does nothing if it does.
	stmt, _ := h.db.BuildInsertIgnoreStmt(h.environment)

	if _, err := h.db.NamedExecContext(h.ctx, stmt, h.environment); err != nil {
		return internal.CantPerformQuery(err, stmt)
	}

	return nil
}

func (h *HA) removeInstance(ctx context.Context) {
	h.logger.Debugw("Removing our row from icingadb_instance", zap.String("instance_id", hex.EncodeToString(h.instanceId)))
	// Intentionally not using h.ctx here as it's already cancelled.
	query := "DELETE FROM icingadb_instance WHERE id = ?"
	_, err := h.db.ExecContext(ctx, query, h.instanceId)
	if err != nil {
		h.logger.Warnw("Could not remove instance from database", zap.Error(err), zap.String("query", query))
	}
}

func (h *HA) removeOldInstances(s *icingaredisv1.IcingaStatus, envId types.Binary) {
	select {
	case <-h.ctx.Done():
		return
	case <-time.After(timeout):
		query := "DELETE FROM icingadb_instance " +
			"WHERE id != ? AND environment_id = ? AND endpoint_id = ? AND heartbeat < ?"
		heartbeat := types.UnixMilli(time.Now().Add(-timeout))
		result, err := h.db.ExecContext(h.ctx, query, h.instanceId, envId,
			s.EndpointId, heartbeat)
		if err != nil {
			h.logger.Errorw("Can't remove rows of old instances", zap.Error(err),
				zap.String("query", query),
				zap.String("id", h.instanceId.String()), zap.String("environment_id", envId.String()),
				zap.String("endpoint_id", s.EndpointId.String()), zap.Time("heartbeat", heartbeat.Time()))
			return
		}
		affected, err := result.RowsAffected()
		if err != nil {
			h.logger.Errorw("Can't get number of removed old instances", zap.Error(err))
			return
		}
		h.logger.Debugf("Removed %d old instances", affected)
	}
}

func (h *HA) signalHandover() {
	if h.responsible {
		select {
		case h.handover <- struct{}{}:
			h.responsible = false
		case <-h.ctx.Done():
			// Noop
		}
	}
}

func (h *HA) signalTakeover() {
	if !h.responsible {
		select {
		case h.takeover <- struct{}{}:
			h.responsible = true
		case <-h.ctx.Done():
			// Noop
		}
	}
}
