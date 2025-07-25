package database

import (
	"context"
	"database/sql/driver"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"sync/atomic"
	"time"
)

// Driver names as automatically registered in the database/sql package by themselves.
const (
	MySQL      string = "mysql"
	PostgreSQL string = "postgres"
)

// OnInitConnFunc can be used to execute post Connect() arbitrary actions.
// It will be called after successfully initiated a new connection using the connector's Connect method.
type OnInitConnFunc func(context.Context, driver.Conn) error

// RetryConnectorCallbacks specifies callbacks that are executed upon certain events.
type RetryConnectorCallbacks struct {
	OnInitConn       OnInitConnFunc
	OnRetryableError retry.OnRetryableErrorFunc
	OnSuccess        retry.OnSuccessFunc
}

// RetryConnector wraps driver.Connector with retry logic.
//
// The first connection attempt will be retried for [retry.DefaultTimeout]. After a prior successful connection,
// reconnection attempts are made infinitely.
type RetryConnector struct {
	driver.Connector

	logger        *logging.Logger
	callbacks     RetryConnectorCallbacks
	hadConnection atomic.Bool
}

// NewConnector creates a fully initialized RetryConnector from the given args.
func NewConnector(c driver.Connector, logger *logging.Logger, callbacks RetryConnectorCallbacks) *RetryConnector {
	return &RetryConnector{Connector: c, logger: logger, callbacks: callbacks}
}

// Connect implements part of the driver.Connector interface.
func (c *RetryConnector) Connect(ctx context.Context) (driver.Conn, error) {
	retryTimeout := retry.DefaultTimeout
	if c.hadConnection.Load() {
		retryTimeout = 0
	}

	var conn driver.Conn
	err := errors.Wrap(retry.WithBackoff(
		ctx,
		func(ctx context.Context) (err error) {
			conn, err = c.Connector.Connect(ctx)
			if err == nil && c.callbacks.OnInitConn != nil {
				if err = c.callbacks.OnInitConn(ctx, conn); err != nil {
					// We're going to retry this, so just don't bother whether Close() fails!
					_ = conn.Close()
				}
			}

			return
		},
		retry.Retryable,
		backoff.DefaultBackoff,
		retry.Settings{
			Timeout: retryTimeout,
			OnRetryableError: func(elapsed time.Duration, attempt uint64, err, lastErr error) {
				if c.callbacks.OnRetryableError != nil {
					c.callbacks.OnRetryableError(elapsed, attempt, err, lastErr)
				}

				c.logger.Warnw("Can't connect to database. Retrying",
					zap.Error(err),
					zap.Duration("after", elapsed),
					zap.Uint64("attempt", attempt))
			},
			OnSuccess: func(elapsed time.Duration, attempt uint64, lastErr error) {
				c.hadConnection.Store(true)

				if c.callbacks.OnSuccess != nil {
					c.callbacks.OnSuccess(elapsed, attempt, lastErr)
				}

				if attempt > 1 {
					c.logger.Infow("Reconnected to database",
						zap.Duration("after", elapsed), zap.Uint64("attempts", attempt))
				}
			},
		},
	), "can't connect to database")
	return conn, err
}

// Driver implements part of the driver.Connector interface.
func (c *RetryConnector) Driver() driver.Driver {
	return c.Connector.Driver()
}

// MysqlFuncLogger is an adapter that allows ordinary functions to be used as a logger for mysql.SetLogger.
type MysqlFuncLogger func(v ...interface{})

// Print implements the mysql.Logger interface.
func (log MysqlFuncLogger) Print(v ...interface{}) {
	log(v)
}
