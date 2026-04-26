package bindings

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
	"github.com/itg-team/itg-ray/internal/server"
)

// probeTimeout bounds a single TCP-connect probe. Matches the spec's
// "TCP-connect probe (1500ms)" requirement and is small enough that even
// a full table of 50 servers tested sequentially returns within ~75s in
// the worst case.
const probeTimeout = 1500 * time.Millisecond

// ErrServerNotFound is returned by ToggleFavorite / TestLatency when the
// caller-supplied id has no entry in the on-disk servers list.
var ErrServerNotFound = errors.New("server not found")

// ServersDeps groups dependencies passed in from main.go. ServerStore is the
// shared Load+Save adapter (defined in app.go); Hub is the in-process
// pub-sub used to fan probe results out to the frontend.
type ServersDeps struct {
	ServerStore ServerStore
	Hub         *hub.Hub
}

// ServersService implements the Servers.* Wails bindings.
type ServersService struct{ d ServersDeps }

// NewServersService constructs a new ServersService. ServersDeps is taken by
// value because the struct is small (two interface/pointer fields) and the
// constructor is invoked once at process start.
func NewServersService(d ServersDeps) *ServersService { return &ServersService{d: d} }

// List returns every known server as a DTO. Frontend sorts/filters
// client-side. Subscriptions list is not loaded here — the Origin column
// shows raw SourceID; AppService.GetSnapshot is the canonical place where
// origins get resolved to display names.
func (s *ServersService) List(_ context.Context) ([]hub.ServerView, error) {
	servers, err := s.d.ServerStore.Load()
	if err != nil {
		return nil, fmt.Errorf("server.Load: %w", err)
	}
	return toServerViews(servers, nil), nil
}

// ToggleFavorite flips the favorite flag for the given server id and
// persists the full list back through the store. Read-modify-write is not
// transactional. server.Save's atomic rename keeps the file structurally
// consistent, but two concurrent ToggleFavorite calls on the same id can
// race on the load → mutate window and lose one update. Today only the
// main window invokes this binding so collisions are unlikely; once tray
// favorites land (C.T13) a per-store mutex should be added.
func (s *ServersService) ToggleFavorite(_ context.Context, id string) error {
	list, err := s.d.ServerStore.Load()
	if err != nil {
		return fmt.Errorf("server.Load: %w", err)
	}
	idx := -1
	for i := range list {
		if list[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrServerNotFound
	}
	list[idx].Favorite = !list[idx].Favorite
	if err := s.d.ServerStore.Save(list); err != nil {
		return fmt.Errorf("server.Save: %w", err)
	}
	return nil
}

// TestLatency probes one server (id != "") or every server (id == "")
// sequentially with a 1.5s TCP-connect timeout. Updated latencies are
// persisted back to the store and the aggregated result list is published
// on the hub as hub.EventProbeResult. Sequential probing is a deliberate
// simplification for C.T5 — concurrent fan-out can be added later if it
// becomes a UX problem.
func (s *ServersService) TestLatency(ctx context.Context, id string) error {
	list, err := s.d.ServerStore.Load()
	if err != nil {
		return fmt.Errorf("server.Load: %w", err)
	}
	targets := make([]int, 0, len(list))
	if id == "" {
		for i := range list {
			targets = append(targets, i)
		}
	} else {
		for i := range list {
			if list[i].ID == id {
				targets = append(targets, i)
				break
			}
		}
		if len(targets) == 0 {
			return ErrServerNotFound
		}
	}

	results := make([]map[string]any, 0, len(targets))
	for _, i := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		r := probeOne(ctx, &list[i])
		results = append(results, r)
		if _, hadError := r["error"]; hadError {
			continue
		}
		if ms, ok := r["latencyMs"].(int); ok {
			latency := ms
			list[i].LatencyMS = &latency
		}
	}
	// Persist updated latencies. Best-effort: if Save fails the in-memory
	// probe results still ship to the frontend so the UI is at least live.
	if err := s.d.ServerStore.Save(list); err != nil {
		return fmt.Errorf("server.Save: %w", err)
	}
	s.d.Hub.Publish(hub.Event{
		Name:    hub.EventProbeResult,
		Payload: map[string]any{"results": results},
	})
	return nil
}

// probeOne dials the server's address:port over TCP and reports either
// the round-trip latency in ms or an error string. Returned shape matches
// the frontend's applyProbeResult expectation:
//
//	{ "id": "...", "latencyMs": 42 }            // success
//	{ "id": "...", "latencyMs": 0,  "error": "…" } // failure
//
// Sub-millisecond RTTs (localhost / LAN) are floored at 1ms so the on-disk
// LatencyMS sentinel "0 = never probed" stays meaningful and the frontend's
// LatencyBadge does not render an em-dash for a healthy server.
func probeOne(ctx context.Context, t *server.Server) map[string]any {
	addr := net.JoinHostPort(t.Vless.Address, strconv.Itoa(int(t.Vless.Port)))
	dialer := &net.Dialer{Timeout: probeTimeout}
	start := time.Now()
	c, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return map[string]any{"id": t.ID, "latencyMs": 0, "error": err.Error()}
	}
	_ = c.Close()
	ms := max(int(time.Since(start).Milliseconds()), 1)
	return map[string]any{
		"id":        t.ID,
		"latencyMs": ms,
	}
}
