package driver

import (
	"context"
	"database/sql/driver"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"time"
)

// Driver names as automatically registered in the database/sql package by themselves.
const (
	MySQL      string = "mysql"
	PostgreSQL string = "postgres"
)

var timeout = time.Minute * 5

// RetryConnector wraps driver.Connector with retry logic.
type RetryConnector struct {
	driver.Connector

	onError   retry.OnErrorFunc
	onSuccess retry.OnSuccessFunc

	logger *logging.Logger
}

// NewConnector creates a fully initialized RetryConnector from the given args.
func NewConnector(c driver.Connector, logger *logging.Logger) *RetryConnector {
	return &RetryConnector{Connector: c, logger: logger}
}

// Connect implements part of the driver.Connector interface.
func (c RetryConnector) Connect(ctx context.Context) (driver.Conn, error) {
	var conn driver.Conn
	err := errors.Wrap(retry.WithBackoff(
		ctx,
		func(ctx context.Context) (err error) {
			conn, err = c.Connector.Connect(ctx)
			return
		},
		shouldRetry,
		backoff.NewExponentialWithJitter(time.Millisecond*128, time.Minute*1),
		retry.Settings{
			Timeout: timeout,
			OnError: func(elapsed time.Duration, attempt uint64, err, lastErr error) {
				if c.onError != nil {
					c.onError(elapsed, attempt, err, lastErr)
				}

				if lastErr == nil || err.Error() != lastErr.Error() {
					c.logger.Warnw("Can't connect to database. Retrying", zap.Error(err))
				}
			},
			OnSuccess: func(elapsed time.Duration, attempt uint64, lastErr error) {
				if c.onSuccess != nil {
					c.onSuccess(elapsed, attempt, lastErr)
				}

				if attempt > 0 {
					c.logger.Infow("Reconnected to database",
						zap.Duration("after", elapsed), zap.Uint64("attempts", attempt+1))
				}
			},
		},
	), "can't connect to database")
	return conn, err
}

// Driver implements part of the driver.Connector interface.
func (c RetryConnector) Driver() driver.Driver {
	return c.Connector.Driver()
}

// Register sets the default mysql logger to the given one.
func Register(logger *logging.Logger) {
	_ = mysql.SetLogger(mysqlLogger(func(v ...interface{}) { logger.Debug(v...) }))
}

// mysqlLogger is an adapter that allows ordinary functions to be used as a logger for mysql.SetLogger.
type mysqlLogger func(v ...interface{})

// Print implements the mysql.Logger interface.
func (log mysqlLogger) Print(v ...interface{}) {
	log(v)
}

func shouldRetry(err error) bool {
	if errors.Is(err, driver.ErrBadConn) {
		return true
	}

	return retry.Retryable(err)
}
