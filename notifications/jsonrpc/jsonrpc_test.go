package jsonrpc

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"testing/synctest"
	"time"

	"github.com/icinga/icinga-go-library/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

func TestJsonRPC(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t).Sugar()

	t.Run("Working RPC Conn", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			serverReader, clientWriter := io.Pipe()
			clientReader, serverWriter := io.Pipe()
			_ = startRPCServer(t.Context(), serverReader, serverWriter)

			client := New(t.Context(), clientReader, clientWriter, nil, logger)

			var result string
			require.NoError(t, client.Call(t.Context(), "ping", nil, &result))
			assert.Equal(t, "pong", result)

			sleepParams := struct {
				Duration time.Duration `json:"duration"`
			}{Duration: 30 * time.Second}

			require.NoError(t, client.Call(t.Context(), "sleep", sleepParams, &result))
			assert.Equal(t, "30s", result)

			require.ErrorContains(t, client.Call(t.Context(), "unknown", nil, &result), "method not found")
			assert.NoError(t, client.Conn().Close())
			synctest.Wait()
		})
	})

	t.Run("Hanging RPC Conn", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			serverReader, clientWriter := io.Pipe()
			clientReader, serverWriter := io.Pipe()

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			_ = startRPCServer(ctx, serverReader, serverWriter)
			client := New(ctx, clientReader, clientWriter, testHandler{}, nil)

			go func() {
				assert.NoError(t, client.NotifyLog(ctx, zapcore.InfoLevel, "test log message", "key", "value"))
				assert.NoError(t, client.NotifyLog(ctx, zapcore.WarnLevel, "test log message 2", "key2", "value2"))
				assert.NoError(t, client.NotifyLog(ctx, zapcore.ErrorLevel, "test log message 3", "key3", "value3"))
			}()

			assert.ErrorIs(t, client.Call(ctx, "no_answer", nil, nil), context.DeadlineExceeded)
			synctest.Wait()
		})
	})

	t.Run("Server Side Close", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			serverReader, clientWriter := io.Pipe()
			clientReader, serverWriter := io.Pipe()

			serverCtx, serverCancel := context.WithCancel(t.Context())
			defer serverCancel()

			ep := startRPCServer(serverCtx, serverReader, serverWriter)
			client := New(t.Context(), clientReader, clientWriter, nil, logger)

			var result string
			require.NoError(t, client.Call(t.Context(), "ping", nil, &result))
			assert.Equal(t, "pong", result)

			serverCancel()
			time.Sleep(5 * time.Second)
			assert.ErrorIs(t, client.Call(t.Context(), "something", nil, &result), ErrClosed)
			assert.ErrorIs(t, ep.Conn().Close(), ErrClosed)

			assert.ErrorIs(t, client.Call(t.Context(), "ping", nil, &result), ErrClosed)
			synctest.Wait()
		})
	})

	t.Run("Async RPC Calls", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			serverReader, clientWriter := io.Pipe()
			clientReader, serverWriter := io.Pipe()

			_ = startRPCServer(t.Context(), serverReader, serverWriter)

			client := New(t.Context(), clientReader, clientWriter, &testHandler{}, logger)
			assert.NoError(t, client.Call(t.Context(), "async_do", nil, nil))
			assert.NoError(t, client.Call(t.Context(), "async_do", nil, nil))
			assert.NoError(t, client.Conn().Close())
			synctest.Wait()
		})
	})
}

// startRPCServer starts a JSON-RPC server with the given context, reader, and writer.
func startRPCServer(ctx context.Context, r io.ReadCloser, w io.WriteCloser) *Endpoint {
	ep := New(ctx, r, w, testHandler{}, zap.NewNop().Sugar())

	go func() {
		defer func() { _ = ep.Conn().Close() }()
		select {
		case <-ctx.Done():
			return
		case <-ep.Done():
			return
		}
	}()
	return ep
}

type testHandler struct{}

func (testHandler) Handle(ctx context.Context, conn *Conn, req *Request) {
	switch req.Method {
	case "ping":
		if err := conn.Reply(ctx, req.ID, "pong"); err != nil {
			panic(err)
		}

	case "sleep":
		var params struct {
			Duration time.Duration `json:"duration"`
		}
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			if errr := ReplyError(ctx, conn, req.ID, CodeInvalidParams, err.Error()); errr != nil {
				panic(errr)
			}
			return
		}
		time.Sleep(params.Duration)

		if err := conn.Reply(ctx, req.ID, params.Duration.String()); err != nil {
			panic(err)
		}

	case "no_answer":
		// Do nothing, just hang the request.

	case "async_do":
		var result string
		if err := conn.Call(ctx, "ping", nil, &result); err != nil {
			panic(err)
		}
		if result != "pong" {
			panic(result)
		}
		if err := conn.Reply(ctx, req.ID, nil); err != nil {
			panic(err)
		}

	case notifications.MethodLog:
		// Received log notification from client, no reply needed.

	default:
		if err := ReplyMethodNotFound(ctx, conn, req.ID); err != nil {
			panic(err)
		}
	}
}
