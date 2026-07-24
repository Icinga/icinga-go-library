//go:build DebugJsonRpc

package jsonrpc

import (
	"encoding/json"
	"log/slog"
)

// jsonrpcDebug is a struct used for logging JSON-RPC request and response pairs for debugging purposes.
type jsonrpcDebug struct {
	request  *Request
	response *Response
}

// LogValue implements the [slog.LogValuer] interface, allowing jsonrpcDebug to be logged as a structured value.
func (j jsonrpcDebug) LogValue() slog.Value {
	req, err := json.Marshal(j.request)
	if err != nil {
		return slog.StringValue("request marshal error: " + err.Error())
	}

	resp, err := json.Marshal(j.response)
	if err != nil {
		return slog.StringValue("resp marshal error: " + err.Error())
	}

	return slog.GroupValue(
		slog.Any("request", json.RawMessage(req)),
		slog.Any("response", json.RawMessage(resp)),
	)
}

// onJsonRpcSend is a callback function used to intercept and log outgoing JSON-RPC messages for debugging purposes.
func onJsonRpcSend(req *Request, resp *Response) {
	slog.Info("JSON-RPC", "send", jsonrpcDebug{request: req, response: resp})
}

// onJsonRpcRecv is a callback function used to intercept and log incoming JSON-RPC messages for debugging purposes.
func onJsonRpcRecv(req *Request, resp *Response) {
	slog.Info("JSON-RPC", "receive", jsonrpcDebug{request: req, response: resp})
}
