package main

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/itg-team/itg-ray/internal/helper/client"
	"github.com/itg-team/itg-ray/internal/helper/protocol"
	"github.com/itg-team/itg-ray/internal/logstream"
)

// lazyLogReader satisfies logstream.LogReader by dialing the helper transport
// (named pipe on Windows, unix socket on Linux) on demand and reusing the
// connection across polls. A failed Call drops the connection so the next poll
// redials — the helper may not be running when the Logs page first opens.
type lazyLogReader struct {
	addr string
	mu   sync.Mutex
	c    *client.Client
}

func newLogReader(addr string) logstream.LogReader { return &lazyLogReader{addr: addr} }

func (r *lazyLogReader) Call(ctx context.Context, op protocol.Op, args json.RawMessage) (json.RawMessage, error) {
	r.mu.Lock()
	if r.c == nil {
		c, err := client.Dial(ctx, r.addr)
		if err != nil {
			r.mu.Unlock()
			return nil, err
		}
		r.c = c
	}
	c := r.c
	r.mu.Unlock()

	raw, err := c.Call(ctx, op, args)
	if err != nil {
		r.mu.Lock()
		if r.c == c {
			_ = r.c.Close()
			r.c = nil
		}
		r.mu.Unlock()
	}
	return raw, err
}
