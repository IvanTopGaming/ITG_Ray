package bindings

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/itg-team/itg-ray/internal/hub"
	"github.com/itg-team/itg-ray/internal/server"
	"github.com/itg-team/itg-ray/internal/vless"
)

// probeTimeout bounds a single TCP-connect probe. Matches the spec's
// "TCP-connect probe (1500ms)" requirement and is small enough that even
// a full table of 50 servers tested sequentially returns within ~75s in
// the worst case.
const probeTimeout = 1500 * time.Millisecond

// ErrServerNotFound is returned by ToggleFavorite / TestLatency when the
// caller-supplied id has no entry in the on-disk servers list.
var ErrServerNotFound = errors.New("server not found")

// ActiveServerProbe is the narrow read used by Remove to block deletion
// of the currently-connected server. *chainctl.Controller satisfies it
// via its ActiveServerID method.
type ActiveServerProbe interface {
	ActiveServerID() string
}

// ServersDeps groups dependencies passed in from main.go. ServerStore is the
// shared Load+Save adapter (defined in app.go); Hub is the in-process
// pub-sub used to fan probe results out to the frontend.
type ServersDeps struct {
	ServerStore  ServerStore
	Hub          *hub.Hub
	ActiveServer ActiveServerProbe // used by Remove; may be nil in tests that don't exercise Remove
	// SubStore lets Remove tell a managed subscription server (parent sub
	// still exists → read-only) from an orphan (parent sub deleted → shown
	// as "manual", must be deletable). nil → orphans can't be detected, so
	// all subscription servers stay read-only (pre-orphan-fix behavior).
	SubStore SubStore
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
func (s *ServersService) List() ([]hub.ServerView, error) {
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
func (s *ServersService) ToggleFavorite(id string) error {
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
	// Publish servers:changed so the frontend's serversStore refetches
	// — favorite is a persisted, list-visible flag; without this the
	// Servers page sees stale state until the next Add/Edit/Remove.
	s.d.Hub.Publish(hub.Event{Name: hub.EventServersChanged})
	return nil
}

// TestLatency probes one server (id != "") or every server (id == "")
// sequentially with a 1.5s TCP-connect timeout. Updated latencies are
// persisted back to the store and the aggregated result list is published
// on the hub as hub.EventProbeResult. Sequential probing is a deliberate
// simplification for C.T5 — concurrent fan-out can be added later if it
// becomes a UX problem.
//
// The Wails binding signature drops ctx (Wails v2.11 does not auto-inject
// it for service methods); a fresh context.Background() is used internally
// for the dialer. The probe loop therefore cannot be cancelled mid-flight
// — accepted tradeoff: each probe is bounded by probeTimeout (1.5s).
func (s *ServersService) TestLatency(id string) error {
	ctx := context.Background()
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
	// Publish servers:changed so the frontend's serversStore refetches
	// the list with new latencies. probe:result drives the dashStore's
	// in-place latency patch (for QuickSwitch sort), but the Servers
	// page's row list is sourced from useServers() and would otherwise
	// drift until the next Add/Edit/Remove.
	s.d.Hub.Publish(hub.Event{Name: hub.EventServersChanged})
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

// validateServerURI parses a VLESS URI and returns the Config. Non-empty
// rawURI is required; vless.ParseURL is the source of truth for what is
// considered well-formed.
func validateServerURI(rawURI string) (vless.Config, error) {
	if rawURI == "" {
		return vless.Config{}, errors.New("uri required")
	}
	cfg, err := vless.ParseURL(rawURI)
	if err != nil {
		return vless.Config{}, fmt.Errorf("invalid VLESS URI: %w", err)
	}
	if _, nerr := cfg.Normalize(); nerr != nil {
		return vless.Config{}, fmt.Errorf("invalid VLESS URI: %w", nerr)
	}
	return cfg, nil
}

// generateServerID returns a fresh manual-server ID of the shape
// m<unix-millis>-<hex4>. Mirrors generateSubID.
func generateServerID() string {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("m%d", time.Now().UnixMilli())
	}
	return fmt.Sprintf("m%d-%s", time.Now().UnixMilli(), hex.EncodeToString(b[:]))
}

// Add creates a manual server from the supplied VLESS URI. Name overrides
// the parsed remark when non-empty; otherwise falls back to remark or
// host:port. Returns the resulting view. NO duplicate-detection — user
// is responsible for list cleanliness (mirrors Subs.Add).
func (s *ServersService) Add(rawURI, name string) (hub.ServerView, error) {
	rawURI = strings.TrimSpace(rawURI)
	cfg, err := validateServerURI(rawURI)
	if err != nil {
		return hub.ServerView{}, err
	}
	name = strings.TrimSpace(name)

	displayName := name
	if displayName == "" {
		displayName = cfg.Remark
	}
	if displayName == "" {
		displayName = net.JoinHostPort(cfg.Address, strconv.Itoa(int(cfg.Port)))
	}

	srv := server.Server{
		ID:     generateServerID(),
		Origin: server.OriginManual,
		Name:   displayName,
		Remark: cfg.Remark,
		Vless:  cfg,
	}

	list, err := s.d.ServerStore.Load()
	if err != nil {
		return hub.ServerView{}, fmt.Errorf("server.Load: %w", err)
	}
	list = append(list, srv)
	if err := s.d.ServerStore.Save(list); err != nil {
		return hub.ServerView{}, fmt.Errorf("server.Save: %w", err)
	}

	s.d.Hub.Publish(hub.Event{Name: hub.EventServersChanged})

	return toServerViews([]server.Server{srv}, nil)[0], nil
}

// Remove deletes a manual server. Refuses when:
//   - Origin != OriginManual                  (read-only)
//   - id == active session id                 (disconnect first)
func (s *ServersService) Remove(id string) error {
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
	if idx == -1 {
		return ErrServerNotFound
	}
	// Manual servers are always deletable. A subscription server is
	// normally read-only (managed by sync) — but once its parent
	// subscription is gone it becomes an orphan (the UI resolves its origin
	// to "manual"), and an orphan that can't be deleted is a dead end.
	if list[idx].Origin != server.OriginManual && !s.isOrphanSubServer(list[idx]) {
		return errors.New("only manual servers can be deleted")
	}
	if s.d.ActiveServer != nil && s.d.ActiveServer.ActiveServerID() == id {
		return errors.New("disconnect first to delete this server")
	}

	list = append(list[:idx], list[idx+1:]...)
	if err := s.d.ServerStore.Save(list); err != nil {
		return fmt.Errorf("server.Save: %w", err)
	}

	s.d.Hub.Publish(hub.Event{Name: hub.EventServersChanged})
	return nil
}

// isOrphanSubServer reports whether a subscription-origin server's parent
// subscription no longer exists. Fails CLOSED — a nil SubStore or a load
// error returns false (treat as still-managed) so a transient read failure
// can never let a genuinely-managed server be deleted.
func (s *ServersService) isOrphanSubServer(srv server.Server) bool {
	if srv.Origin != server.OriginSubscription || s.d.SubStore == nil {
		return false
	}
	subs, err := s.d.SubStore.Load()
	if err != nil {
		return false
	}
	for i := range subs {
		if subs[i].ID == srv.SourceID {
			return false // parent still exists → managed, not an orphan
		}
	}
	return true
}

// Edit updates name and/or VLESS config for an existing server. Refuses
// when the target's Origin != OriginManual (sub-origin is read-only —
// managed by sync). When the URI changes the underlying vless.Config is
// re-parsed; the server's ID stays stable so dashStore.currentServer
// references survive the edit.
//
// vlessChanged reports whether the parsed vless.Config differs from the
// pre-edit one (reflect.DeepEqual on vless.Config). Frontend uses this
// flag to decide whether to show the "Reconnect to apply" banner —
// name-only edits get vlessChanged=false and stay silent.
func (s *ServersService) Edit(id, rawURI, name string) (hub.ServerView, bool, error) {
	rawURI = strings.TrimSpace(rawURI)
	cfg, err := validateServerURI(rawURI)
	if err != nil {
		return hub.ServerView{}, false, err
	}
	name = strings.TrimSpace(name)

	list, err := s.d.ServerStore.Load()
	if err != nil {
		return hub.ServerView{}, false, fmt.Errorf("server.Load: %w", err)
	}
	idx := -1
	for i := range list {
		if list[i].ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return hub.ServerView{}, false, ErrServerNotFound
	}
	if list[idx].Origin != server.OriginManual {
		return hub.ServerView{}, false, errors.New("only manual servers can be edited")
	}

	oldVless := list[idx].Vless
	vlessChanged := !reflect.DeepEqual(oldVless, cfg)

	displayName := name
	if displayName == "" {
		displayName = cfg.Remark
	}
	if displayName == "" {
		displayName = net.JoinHostPort(cfg.Address, strconv.Itoa(int(cfg.Port)))
	}

	list[idx].Name = displayName
	list[idx].Remark = cfg.Remark
	list[idx].Vless = cfg
	// Favorite, Disabled, Tags, LatencyMS preserved by skipping them.

	if err := s.d.ServerStore.Save(list); err != nil {
		return hub.ServerView{}, false, fmt.Errorf("server.Save: %w", err)
	}

	s.d.Hub.Publish(hub.Event{Name: hub.EventServersChanged})

	return toServerViews([]server.Server{list[idx]}, nil)[0], vlessChanged, nil
}
