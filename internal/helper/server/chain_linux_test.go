//go:build linux

package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/itg-team/itg-ray/internal/helper/supervisor"
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

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
