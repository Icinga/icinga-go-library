//go:build !DebugJsonRpc

package jsonrpc

// onJsonRpcSend is a no-op stub for the OnJsonRpcSend callback in non-debug builds.
func onJsonRpcSend(req *Request, resp *Response) {}

// onJsonRpcRecv is a no-op stub for the OnJsonRpcRecv callback in non-debug builds.
func onJsonRpcRecv(req *Request, resp *Response) {}
