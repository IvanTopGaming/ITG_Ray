//go:build windows

package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestStartChainArgs_DecodeIncludesMode(t *testing.T) {
	raw := []byte(`{"singbox_config":{},"xray_config":{},"server_host":"x","server_port":1,"tun_name":"t","mode":"sysproxy"}`)
	var a StartChainArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if a.Mode != "sysproxy" {
		t.Fatalf("Mode=%q, want sysproxy", a.Mode)
	}
}

func TestStartChainHandler_InvalidMode(t *testing.T) {
	h := NewStartChainHandler()
	args := json.RawMessage(`{"singbox_config":{},"xray_config":{},"server_host":"x","server_port":1,"tun_name":"t","mode":"bogus"}`)
	_, err := h(context.Background(), args)
	if err == nil || !strings.Contains(err.Error(), "invalid mode") {
		t.Fatalf("err=%v, want 'invalid mode'", err)
	}
}

func TestStartChainHandler_TunModeRequiresTunName(t *testing.T) {
	h := NewStartChainHandler()
	args := json.RawMessage(`{"singbox_config":{},"xray_config":{},"server_host":"x","server_port":1,"mode":"tun"}`)
	_, err := h(context.Background(), args)
	if err == nil || !strings.Contains(err.Error(), "tun_name required") {
		t.Fatalf("err=%v, want tun_name required", err)
	}
}
