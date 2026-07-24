// Package jsonrpc provides a wrapper around the github.com/sourcegraph/jsonrpc2 package to facilitate
// communication between Icinga Notifications and its plugins using the JSON-RPC 2.0 protocol.
//
// The package defines an Endpoint type that represents a JSON-RPC endpoint capable of sending requests
// and notifications over a connection. It also provides utility functions for sending log messages and
// error responses as JSON-RPC notifications. The package re-exports some constants and types from the
// jsonrpc2 package for convenience.
//
// If you are developing a plugin for Icinga Notifications and have problems with the JSON-RPC communication,
// you can build your plugin with the `DebugJsonRpc` build tag to enable debug logging of the JSON-RPC messages
// sent and received by the plugin. The debug logs will contain the raw JSON-RPC messages as received from Icinga
// Notifications, as well as the raw JSON-RPC requests and responses made by the plugin to Icinga Notifications.
package jsonrpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/icinga/icinga-go-library/notifications"
	"github.com/sourcegraph/jsonrpc2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Error codes defined in the JSON-RPC 2.0 specification and re-exported from the jsonrpc2 package for convenience.
const (
	CodeParseError     = jsonrpc2.CodeParseError
	CodeInvalidRequest = jsonrpc2.CodeInvalidRequest
	CodeMethodNotFound = jsonrpc2.CodeMethodNotFound
	CodeInvalidParams  = jsonrpc2.CodeInvalidParams
	CodeInternalError  = jsonrpc2.CodeInternalError
)

// ErrClosed indicates that the JSON-RPC connection has been closed, re-exported from the jsonrpc2 package for convenience.
var ErrClosed = jsonrpc2.ErrClosed

// Type aliases for the jsonrpc2 package types to avoid importing the package directly in other files.
type (
	// Conn is a type alias for jsonrpc2.Conn, representing a raw JSON-RPC connection.
	Conn = jsonrpc2.Conn

	// Handler is a type alias for jsonrpc2.Handler, representing a handler for JSON-RPC requests.
	Handler = jsonrpc2.Handler

	// Request is a type alias for jsonrpc2.Request, representing a JSON-RPC request.
	Request = jsonrpc2.Request

	// Response is a type alias for jsonrpc2.Response, representing a JSON-RPC response.
	Response = jsonrpc2.Response

	// Error is a type alias for jsonrpc2.Error, representing a JSON-RPC error response.
	Error = jsonrpc2.Error
)

type (
	// Endpoint represents a JSON-RPC endpoint that can send requests and notifications over a connection.
	//
	// It can be used by both the client and server sides of a JSON-RPC connection to send and handle RPC messages.
	// The underlying connection is managed by the Conn type, which handles the low-level details of reading and writing
	// JSON-RPC messages and can be accessed via the [Conn] method if needed.
	Endpoint struct {
		conn *Conn

		// logsCh is a channel used to send log messages as JSON-RPC notifications to the other side of the connection.
		// It is only initialized if the endpoint is created with the reportRPCErrorsToPeer option set to true.
		logsCh chan *LogParams
	}

	// LogParams represents a log message sent from a plugin to the Icinga Notifications via a JSON-RPC notification.
	//
	// The fields can be any type that can be serialized to JSON, and will be included in the log entry as key-value
	// pairs. Icinga Notifications won't perform any sanity checks on the resulted log entry, so be sure to not send
	// any sensitive information that you don't want to be logged. You can inspect your plugin's log messages in the
	// Icinga Notifications logging output, which is typically available via the systemd journal under the "channel"
	// log context and can be filtered by the plugin's name or channel type.
	LogParams struct {
		Level   zapcore.Level `json:"level"`
		Message string        `json:"message"`
		Fields  []any         `json:"fields"`
	}
)

// New creates a new JSON-RPC endpoint with the given context, read and write streams, and request handler.
//
// If the handler is nil, the endpoint will function as a client-only endpoint that can send requests or push
// notifications but will not handle incoming RPC requests (panics if a request is received). The read and write
// streams are used for communication with the other side of the JSON-RPC connection, and might be connected to
// a pipe, or any other I/O stream.
//
// However, the plugins are expected to pass in their respective stdin and stdout streams for communication with
// Icinga Notifications. When done, the caller is responsible for closing the RPC connection by calling the
// [Conn.Close] method on the returned endpoint.
//
// If the logger is nil, the endpoint will create an internal channel to send log messages as JSON-RPC notifications
// to the other side of the connection. Otherwise, the provided logger will be used to log messages related to the
// JSON-RPC connection and its messages.
func New(ctx context.Context, r io.ReadCloser, w io.WriteCloser, h Handler, logger *zap.SugaredLogger) *Endpoint {
	opts := []jsonrpc2.ConnOpt{
		jsonrpc2.OnSend(func(req *Request, resp *Response) { onJsonRpcSend(logger, req, resp) }),
		jsonrpc2.OnRecv(func(req *Request, resp *Response) { onJsonRpcRecv(logger, req, resp) }),
	}

	ep := new(Endpoint)
	if logger == nil {
		opts = append(opts, jsonrpc2.SetLogger(loggerFn(ep.NotifyLog)))
	} else {
		logger = logger.With("context", "jsonrpc")
		opts = append(opts, jsonrpc2.SetLogger(loggerFn(func(_ context.Context, lvl zapcore.Level, msg string, fields ...any) error {
			logger.Logw(lvl, msg, fields...)
			return nil
		})))
	}
	ep.conn = jsonrpc2.NewConn(ctx, jsonrpc2.NewPlainObjectStream(&readWriteCloser{r, w}), jsonrpc2.AsyncHandler(h), opts...)
	if logger == nil {
		ep.logsCh = make(chan *LogParams, 64)
		go ep.serveLogNotifications(ctx)
	}
	return ep
}

// Conn returns the underlying JSON-RPC connection.
func (e *Endpoint) Conn() *Conn { return e.conn }

// Done returns a channel that is closed when the underlying connection is closed or the context is canceled.
func (e *Endpoint) Done() <-chan struct{} { return e.conn.DisconnectNotify() }

// Call sends a JSON-RPC request with the given method and params, and returns the result or an error.
func (e *Endpoint) Call(ctx context.Context, method string, params, result any) error {
	return e.conn.Call(ctx, method, params, result)
}

// NotifyLog sends a log message as a JSON-RPC notification to the other side of the connection.
//
// Returns an error if the endpoint does not support logging or if the context is canceled
// before the log message can be sent.
func (e *Endpoint) NotifyLog(ctx context.Context, lvl zapcore.Level, msg string, fields ...any) error {
	if e.logsCh == nil {
		return fmt.Errorf("NotifyLog called on an endpoint that doesn't support logging")
	}

	select {
	default:
		return errors.New("NotifyLog channel is full, dropping log message")
	case <-ctx.Done():
		return ctx.Err()
	case <-e.Done():
		return ErrClosed
	case e.logsCh <- &LogParams{Level: lvl, Message: msg, Fields: fields}:
		return nil
	}
}

// serveLogNotifications is a goroutine that listens for log messages on the logsCh channel and sends them as JSON-RPC
// notifications to the other side of the connection. It runs until the context is canceled or the connection is closed.
func (e *Endpoint) serveLogNotifications(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.Done():
			return
		case log := <-e.logsCh:
			// Don't care about the error here, as we can't do much if the notification fails to send.
			_ = e.conn.Notify(ctx, notifications.MethodLog, log)
		}
	}
}

// ReplyError sends a JSON-RPC error response with the given request ID, error code, and message.
func ReplyError(ctx context.Context, c *Conn, reqID jsonrpc2.ID, code int64, msg string) error {
	return c.ReplyWithError(ctx, reqID, &Error{Code: code, Message: msg})
}

// ReplyMethodNotFound sends a JSON-RPC error response indicating that the requested method was not found.
func ReplyMethodNotFound(ctx context.Context, c *Conn, reqID jsonrpc2.ID) error {
	return ReplyError(ctx, c, reqID, CodeMethodNotFound, "method not found")
}

// ReplyMissingParams sends a JSON-RPC error response indicating that the request is missing required parameters.
func ReplyMissingParams(ctx context.Context, c *Conn, reqID jsonrpc2.ID) error {
	return ReplyError(ctx, c, reqID, CodeInvalidRequest, "missing required parameters")
}

// readWriteCloser is a helper type that combines an [io.ReadCloser] and an [io.WriteCloser] into an [io.ReadWriteCloser].
type readWriteCloser struct {
	r io.ReadCloser
	w io.WriteCloser
}

func (rwc *readWriteCloser) Read(p []byte) (n int, err error)  { return rwc.r.Read(p) }
func (rwc *readWriteCloser) Write(p []byte) (n int, err error) { return rwc.w.Write(p) }
func (rwc *readWriteCloser) Close() error                      { return errors.Join(rwc.r.Close(), rwc.w.Close()) }

// loggerFn is a helper type that implements the [jsonrpc2.Logger] interface by sending log messages as JSON-RPC notifications.
type loggerFn func(ctx context.Context, lvl zapcore.Level, msg string, fields ...any) error

func (fn loggerFn) Printf(format string, v ...any) {
	lvl := zapcore.DebugLevel
	if strings.Contains(format, "error") {
		lvl = zapcore.ErrorLevel
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = fn(ctx, lvl, fmt.Sprintf(format, v...))
}
