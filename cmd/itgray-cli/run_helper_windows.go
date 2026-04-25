//go:build windows

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/itg-team/itg-ray/internal/helper/client"
	"github.com/itg-team/itg-ray/internal/helper/protocol"
	"github.com/itg-team/itg-ray/internal/helper/route"
	helperserver "github.com/itg-team/itg-ray/internal/helper/server"
	"github.com/itg-team/itg-ray/internal/server"
)

const helperPipe = `\\.\pipe\ITGRay.Helper.v1`

func startHelperSession(ctx context.Context, _ *server.Server, tunName, _ string) (cleanup func(), luid uint64, err error) {
	c, err := client.Dial(ctx, helperPipe)
	if err != nil {
		return nil, 0, fmt.Errorf("dial helper pipe: %w", err)
	}

	// Snapshot routes for restore.
	rawSnap, err := c.Call(ctx, protocol.OpRouteSnapshot, json.RawMessage(`{}`))
	if err != nil {
		_ = c.Close()
		return nil, 0, fmt.Errorf("RouteSnapshot: %w", err)
	}
	var snap helperserver.RouteSnapshotResult
	if err := json.Unmarshal(rawSnap, &snap); err != nil {
		_ = c.Close()
		return nil, 0, fmt.Errorf("decode snapshot: %w", err)
	}

	// Create TUN adapter.
	tunArgs, _ := json.Marshal(helperserver.TunCreateArgs{Name: tunName})
	rawTun, err := c.Call(ctx, protocol.OpTunCreate, tunArgs)
	if err != nil {
		_ = c.Close()
		return nil, 0, fmt.Errorf("TunCreate: %w", err)
	}
	var tunRes helperserver.TunCreateResult
	if err := json.Unmarshal(rawTun, &tunRes); err != nil {
		_ = c.Close()
		return nil, 0, fmt.Errorf("decode TunCreate: %w", err)
	}

	// Add catch-all route via TUN.
	addArgs, _ := json.Marshal(route.Entry{
		DestCIDR:      "0.0.0.0/0",
		NextHop:       "0.0.0.0",
		InterfaceLUID: tunRes.LUID,
		Metric:        0,
	})
	destroyArgs, _ := json.Marshal(helperserver.TunDestroyArgs{Name: tunName})
	if _, err := c.Call(ctx, protocol.OpRouteAdd, addArgs); err != nil {
		_, _ = c.Call(ctx, protocol.OpTunDestroy, destroyArgs)
		_ = c.Close()
		return nil, 0, fmt.Errorf("RouteAdd: %w", err)
	}

	cleanup = func() {
		// Restore routes first — TunDestroy invalidates the LUID.
		restoreArgs, _ := json.Marshal(struct {
			Snapshot []route.Entry `json:"snapshot"`
		}{Snapshot: snap.Routes})
		_, _ = c.Call(ctx, protocol.OpRouteRestore, restoreArgs)
		// Destroy TUN.
		_, _ = c.Call(ctx, protocol.OpTunDestroy, destroyArgs)
		_ = c.Close()
	}
	return cleanup, tunRes.LUID, nil
}
