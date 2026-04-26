// Package server is the cross-platform Helper server: dispatcher + handler
// registry. The Windows pipe listener (pipe_windows.go) and SID auth
// (auth_windows.go) wrap it; Linux tests drive it directly via net.Pipe.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/itg-team/itg-ray/internal/helper/protocol"
)

// Handler is invoked by the dispatcher for one request body.
type Handler func(ctx context.Context, args json.RawMessage) (json.RawMessage, error)

// Dispatcher is the cross-platform request router.
type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[protocol.Op]Handler
}

// NewDispatcher returns an empty Dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{handlers: map[protocol.Op]Handler{}}
}

// Register associates an Op with its handler. The last registration for a
// given Op wins.
func (d *Dispatcher) Register(op protocol.Op, h Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[op] = h
}

// Serve runs one client connection until EOF or ctx cancel. Each request is
// dispatched serially per connection; handlers may run blocking work.
func (d *Dispatcher) Serve(ctx context.Context, conn net.Conn) {
	defer conn.Close() //nolint:errcheck // close on exit; error irrelevant
	for {
		body, err := protocol.ReadFrame(conn, protocol.MaxFrame)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
				slog.Debug("helper: read frame", slog.String("scope", "helper.server"), slog.String("err", err.Error()))
			}
			return
		}
		var req protocol.Request
		if err := json.Unmarshal(body, &req); err != nil {
			d.write(conn, protocol.NewError(0, fmt.Sprintf("invalid request: %v", err)))
			continue
		}
		d.handle(ctx, conn, req)
	}
}

func (d *Dispatcher) handle(ctx context.Context, conn net.Conn, req protocol.Request) {
	d.mu.RLock()
	h, ok := d.handlers[req.Op]
	d.mu.RUnlock()
	if !ok {
		d.write(conn, protocol.NewError(req.ID, fmt.Sprintf("unknown op: %s", req.Op)))
		return
	}
	res, err := h(ctx, req.Args)
	if err != nil {
		d.write(conn, protocol.NewError(req.ID, err.Error()))
		return
	}
	d.write(conn, protocol.NewOK(req.ID, res))
}

func (d *Dispatcher) write(conn net.Conn, resp protocol.Response) {
	body, err := json.Marshal(resp)
	if err != nil {
		slog.Error("helper: marshal response", slog.String("err", err.Error()))
		return
	}
	if err := protocol.WriteFrame(conn, body); err != nil {
		slog.Debug("helper: write frame", slog.String("err", err.Error()))
	}
}
