//go:build linux

package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/helper/supervisor"
	"github.com/itg-team/itg-ray/internal/logtest"
)

func TestStartChain_RejectsMissingServer(t *testing.T) {
	h := NewStartChainHandler()
	_, err := h(context.Background(), mustJSON(t, StartChainArgs{TunName: "t", Mode: "tun"}))
	if err == nil {
		t.Fatal("expected error when server_host/server_port missing")
	}
}

func TestStartChain_SpawnFailureRollsBack(t *testing.T) {
	orig := spawnCore
	spawnCore = func(name, exe string, args []string, logPath string) (*supervisor.Child, error) {
		return nil, errors.New("boom")
	}
	defer func() { spawnCore = orig }()

	h := NewStartChainHandler()
	_, err := h(context.Background(), mustJSON(t, StartChainArgs{
		SingboxConfig: json.RawMessage(`{}`), XrayConfig: json.RawMessage(`{}`),
		ServerHost: "203.0.113.7", ServerPort: 443, TunName: "ITGRay-TUN", Mode: "tun",
	}))
	if err == nil {
		t.Fatal("expected StartChain to fail when spawn fails")
	}
	if IsChainActive() {
		t.Fatal("no chain should be active after a failed StartChain")
	}
}

func TestStartChain_RollbackWarnsOnSwallowedCoreStopFailure(t *testing.T) {
	buf := logtest.Capture(t)

	origSpawn := spawnCore
	spawnCore = func(name, exe string, args []string, logPath string) (*supervisor.Child, error) {
		if name == "sing-box" {
			return &supervisor.Child{}, nil
		}
		return nil, errors.New("xray spawn boom")
	}
	defer func() { spawnCore = origSpawn }()

	origStop := stopChildBestEffort
	stopChildBestEffort = func(c *supervisor.Child, grace time.Duration) error {
		return errors.New("stop boom")
	}
	defer func() { stopChildBestEffort = origStop }()

	h := NewStartChainHandler()
	_, err := h(context.Background(), mustJSON(t, StartChainArgs{
		SingboxConfig: json.RawMessage(`{}`), XrayConfig: json.RawMessage(`{}`),
		ServerHost: "203.0.113.7", ServerPort: 443, TunName: "ITGRay-TUN", Mode: "tun",
	}))
	if err == nil {
		t.Fatal("expected StartChain to fail when xray spawn fails")
	}
	if IsChainActive() {
		t.Fatal("no chain should be active after a failed StartChain")
	}

	out := buf.String()
	if !strings.Contains(out, "[helper]") {
		t.Fatalf("missing helper scope in log: %q", out)
	}
	if !strings.Contains(out, "chain teardown: singbox stop failed") {
		t.Fatalf("swallowed rollback stop failure not logged: %q", out)
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
