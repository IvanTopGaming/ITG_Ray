//go:build windows

package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/itg-team/itg-ray/internal/helper/route"
)

// RouteSnapshotResult is what OpRouteSnapshot returns.
type RouteSnapshotResult struct {
	Routes []route.Entry `json:"routes"`
}

// NewRouteSnapshotHandler reads the current route table.
func NewRouteSnapshotHandler() Handler {
	return func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
		entries, err := route.Snapshot()
		if err != nil {
			return nil, err
		}
		return json.Marshal(RouteSnapshotResult{Routes: entries})
	}
}

// NewRouteAddHandler installs one route entry.
func NewRouteAddHandler() Handler {
	return func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		var e route.Entry
		if err := json.Unmarshal(args, &e); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
		if err := route.Add(e); err != nil {
			return nil, err
		}
		return json.RawMessage(`{}`), nil
	}
}

// NewRouteRemoveHandler removes one route entry.
func NewRouteRemoveHandler() Handler {
	return func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		var e route.Entry
		if err := json.Unmarshal(args, &e); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
		if err := route.Remove(e); err != nil {
			return nil, err
		}
		return json.RawMessage(`{}`), nil
	}
}

// NewRouteRestoreHandler accepts a previously-captured snapshot and walks
// the diff: routes present in current but not in snapshot are removed;
// routes present in snapshot but not in current are added. The snapshot is
// supplied by the user-level CLI from its undo journal — the handler does
// not persist anything itself.
func NewRouteRestoreHandler() Handler {
	return func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		var payload struct {
			Snapshot []route.Entry `json:"snapshot"`
		}
		if err := json.Unmarshal(args, &payload); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
		current, err := route.Snapshot()
		if err != nil {
			return nil, err
		}
		want := indexEntries(payload.Snapshot)
		have := indexEntries(current)

		// Remove routes in have but not in want.
		for k, e := range have {
			if _, keep := want[k]; !keep {
				_ = route.Remove(e) // best-effort; transient mismatches are OK
			}
		}
		// Add routes in want but not in have.
		for k, e := range want {
			if _, exists := have[k]; !exists {
				_ = route.Add(e)
			}
		}
		return json.RawMessage(`{}`), nil
	}
}

func indexEntries(es []route.Entry) map[string]route.Entry {
	out := make(map[string]route.Entry, len(es))
	for _, e := range es {
		key := fmt.Sprintf("%s|%s|%d", e.DestCIDR, e.NextHop, e.InterfaceLUID)
		out[key] = e
	}
	return out
}
