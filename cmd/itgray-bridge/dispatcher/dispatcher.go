package dispatcher

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"
)

// Handler implements one JSON-RPC method. Receives parsed params; returns
// the result value (encoded into Response.Result) or an error. If the error
// is *Error it's returned verbatim; otherwise it's wrapped as Internal.
type Handler func(ctx context.Context, params json.RawMessage) (any, error)

// Dispatcher serves JSON-RPC requests from a Reader to a Writer. Safe for
// concurrent registration before Serve is called; Serve itself is single-
// threaded (one request at a time, response written before next read) which
// matches the stdin/stdout transport's natural ordering guarantees.
type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[string]Handler
	Observer func(method string, params json.RawMessage, err error, dur time.Duration)
}

// New returns a Dispatcher with an empty handler map.
func New() *Dispatcher {
	return &Dispatcher{handlers: make(map[string]Handler)}
}

// Register associates method with a handler. Panics if method is already
// registered (a programming error caught at startup).
func (d *Dispatcher) Register(method string, h Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, dup := d.handlers[method]; dup {
		panic("dispatcher: duplicate method " + method)
	}
	d.handlers[method] = h
}

// Serve reads newline-delimited JSON-RPC requests from r and writes responses
// to w. Returns nil on EOF. Returns the underlying read error on non-EOF
// failure. Each line is one Request; malformed JSON produces a parse-error
// Response with id null per JSON-RPC spec.
func (d *Dispatcher) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1 MiB max line
	enc := json.NewEncoder(w)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		resp := d.handle(ctx, line)
		if resp == nil { // notification — no response
			continue
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func (d *Dispatcher) handle(ctx context.Context, line []byte) *Response {
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      json.RawMessage("null"),
			Error:   &Error{Code: CodeParseError, Message: "invalid JSON"},
		}
	}
	if req.JSONRPC != JSONRPCVersion {
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   &Error{Code: CodeInvalidRequest, Message: "bad jsonrpc version"},
		}
	}
	if req.ID == nil { // notification from main — ignored
		return nil
	}
	d.mu.RLock()
	h, ok := d.handlers[req.Method]
	d.mu.RUnlock()
	if !ok {
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   &Error{Code: CodeMethodNotFound, Message: "method not found: " + req.Method},
		}
	}
	start := time.Now()
	result, err := h(ctx, req.Params)
	if d.Observer != nil {
		d.Observer(req.Method, req.Params, err, time.Since(start))
	}
	if err != nil {
		var jerr *Error
		if errors.As(err, &jerr) {
			return &Response{JSONRPC: JSONRPCVersion, ID: req.ID, Error: jerr}
		}
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   &Error{Code: CodeInternal, Message: err.Error()},
		}
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   &Error{Code: CodeInternal, Message: "marshal result: " + err.Error()},
		}
	}
	return &Response{JSONRPC: JSONRPCVersion, ID: req.ID, Result: raw}
}

// Error implements the error interface so handlers can return *Error directly.
func (e *Error) Error() string { return e.Message }
