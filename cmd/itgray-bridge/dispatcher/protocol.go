// Package dispatcher reads JSON-RPC 2.0 requests line-by-line from an
// io.Reader and writes responses + notifications to an io.Writer.
// Designed for stdin/stdout transport between Electron main and itgray-bridge.
package dispatcher

import "encoding/json"

// Request is an inbound JSON-RPC request from Electron main.
// ID is omitted by the wire encoder when nil — that case is a notification
// from main, but in practice main does not send notifications, so
// dispatcher only handles requests with non-nil ID.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // omitempty so notifications elide it
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is an outbound success/error reply to a Request.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Notification is an outbound bridge → main event (no ID).
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Error follows JSON-RPC 2.0 error object shape.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Standard JSON-RPC error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternal       = -32603
)

// JSONRPCVersion is the protocol literal embedded in every message.
const JSONRPCVersion = "2.0"
