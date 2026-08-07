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

// maxConcurrentRequests caps how many handlers may run at once. Requests
// beyond it queue at the read loop (backpressure) rather than spawning
// unbounded goroutines. Generous on purpose: the only client is our own UI,
// and the slow methods (subs.syncOne at up to 30s per fetch) must not starve
// quick ones like rules.list.
const maxConcurrentRequests = 32

// Dispatcher serves JSON-RPC requests from a Reader to a Writer. Safe for
// concurrent registration before Serve is called. Serve runs handlers
// concurrently (up to maxConcurrentRequests): responses carry the request id,
// so the client matches them without relying on arrival order, and a slow
// method must not hold up everything queued behind it.
//
// Handlers therefore must be safe to call concurrently. Today they are: the
// chain controller, the rules service, the config store and the log buffer
// each hold their own lock, and the two unsynchronized read-modify-write
// paths (servers.json / subscriptions.json) are serialized by the shared
// bindings.StoreLock.
type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[string]Handler
	// Observer, when set, is called once per handled request. It runs on the
	// handler's goroutine, so it must be safe for concurrent use.
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

// Serve reads newline-delimited JSON-RPC requests from r and dispatches each
// to its handler in its own goroutine, writing responses to w as they finish.
// Returns nil on EOF, once every in-flight handler has completed and its
// response has been written. Returns the underlying read error on non-EOF
// failure, or the first response-write error. Each line is one Request;
// malformed JSON produces a parse-error Response with id null per JSON-RPC
// spec.
//
// Responses are emitted in completion order, not request order — the id in
// each Response is what pairs it with its request.
func (d *Dispatcher) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1 MiB max line
	enc := json.NewEncoder(w)

	var (
		wg    sync.WaitGroup
		encMu sync.Mutex // guards enc and writeErr
		// writeErr holds the first failed response write (a closed stdout,
		// typically). The read loop stops at the next line so the bridge
		// winds down instead of spinning on a dead pipe.
		writeErr error
	)
	sem := make(chan struct{}, maxConcurrentRequests)

	for scanner.Scan() {
		encMu.Lock()
		stop := writeErr != nil
		encMu.Unlock()
		if stop {
			break
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		// scanner reuses its buffer on the next Scan, so the handler
		// goroutine needs its own copy of the request bytes.
		req := make([]byte, len(line))
		copy(req, line)

		sem <- struct{}{} // backpressure once maxConcurrentRequests are busy
		wg.Go(func() {
			defer func() { <-sem }()
			resp := d.handle(ctx, req)
			if resp == nil { // notification — no response
				return
			}
			encMu.Lock()
			defer encMu.Unlock()
			if writeErr != nil {
				return
			}
			if err := enc.Encode(resp); err != nil {
				writeErr = err
			}
		})
	}
	wg.Wait()

	if writeErr != nil {
		return writeErr
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
