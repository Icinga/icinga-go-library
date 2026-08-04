//go:build !DebugJsonRpc

package jsonrpc

import "go.uber.org/zap"

// onJsonRpcSend is a no-op stub for the OnJsonRpcSend callback in non-debug builds.
func onJsonRpcSend(*zap.SugaredLogger, *Request, *Response) {}

// onJsonRpcRecv is a no-op stub for the OnJsonRpcRecv callback in non-debug builds.
func onJsonRpcRecv(*zap.SugaredLogger, *Request, *Response) {}
