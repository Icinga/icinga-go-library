// Package jsonrpc provides a wrapper around the github.com/sourcegraph/jsonrpc2 package to facilitate
// communication between Icinga Notifications and its plugins using the JSON-RPC 2.0 protocol.
//
// The package defines an Endpoint type that represents a JSON-RPC endpoint capable of sending requests
// and notifications over a connection. It also provides utility functions for sending log messages and
// error responses as JSON-RPC notifications. The package re-exports some constants and types from the
// jsonrpc2 package for convenience.
package jsonrpc

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/sourcegraph/jsonrpc2"
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
)

type (
	// Endpoint represents a JSON-RPC endpoint that can send requests and notifications over a connection.
	//
	// It can be used by both the client and server sides of a JSON-RPC connection to send and handle RPC messages.
	// The underlying connection is managed by the Conn type, which handles the low-level details of reading and writing
	// JSON-RPC messages and can be accessed via the [Conn] method if needed.
	Endpoint struct {
		conn *Conn
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
// notifications but will not handle incoming RPC requests. The read and write streams are used for communication
// with the other side of the JSON-RPC connection, and might be connected to a pipe, or any other I/O stream.
//
// However, the plugins are expected to pass in their respective stdin and stdout streams for communication with
// Icinga Notifications. When done, the caller is responsible for closing the RPC connection by calling the
// [Conn.Close] method on the returned endpoint.
func New(ctx context.Context, r io.ReadCloser, w io.WriteCloser, h Handler) *Endpoint {
	ep := new(Endpoint)
	ep.conn = jsonrpc2.NewConn(
		ctx,
		jsonrpc2.NewPlainObjectStream(&readWriteCloser{r, w}),
		h,
		jsonrpc2.SetLogger(loggerFn(ep.NotifyLog)),
		jsonrpc2.OnSend(onJsonRpcSend),
		jsonrpc2.OnRecv(onJsonRpcRecv),
	)
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

// Notify sends a JSON-RPC notification with the given method and params.
func (e *Endpoint) Notify(ctx context.Context, method string, params any) error {
	return e.conn.Notify(ctx, method, params)
}

// NotifyLog sends a JSON-RPC notification with the given log level, message, and fields.
//
// The log message will be handled by the Icinga Notifications logging system and can be viewed in the
// systemd journal or other logging output. See the docstring of [LogParams] for more details.
func (e *Endpoint) NotifyLog(ctx context.Context, lvl zapcore.Level, msg string, fields ...any) error {
	return e.Notify(ctx, "Log", &LogParams{Level: lvl, Message: msg, Fields: fields})
}

// ReplyError sends a JSON-RPC error response with the given request ID, error code, and message.
func ReplyError(ctx context.Context, c *Conn, reqID jsonrpc2.ID, code int64, msg string) error {
	return c.ReplyWithError(ctx, reqID, &jsonrpc2.Error{Code: code, Message: msg})
}

// ReplyMethodNotFound sends a JSON-RPC error response indicating that the requested method was not found.
func ReplyMethodNotFound(ctx context.Context, c *Conn, reqID jsonrpc2.ID) error {
	return ReplyError(ctx, c, reqID, CodeMethodNotFound, "method not found")
}

// readWriteCloser is a helper type that combines an [io.ReadCloser] and an [io.WriteCloser] into an [io.ReadWriteCloser].
type readWriteCloser struct {
	r io.ReadCloser
	w io.WriteCloser
}

func (rwc *readWriteCloser) Read(p []byte) (n int, err error)  { return rwc.r.Read(p) }
func (rwc *readWriteCloser) Write(p []byte) (n int, err error) { return rwc.w.Write(p) }
func (rwc *readWriteCloser) Close() error {
	if err := rwc.r.Close(); err != nil {
		return err
	}
	return rwc.w.Close()
}

// loggerFn is a helper type that implements the [jsonrpc2.Logger] interface by sending log messages as JSON-RPC notifications.
type loggerFn func(ctx context.Context, lvl zapcore.Level, msg string, fields ...any) error

func (fn loggerFn) Printf(format string, v ...any) {
	lvl := zapcore.DebugLevel
	if strings.Contains(format, "error") {
		lvl = zapcore.ErrorLevel
	}
	_ = fn(context.Background(), lvl, fmt.Sprintf(format, v...))
}
