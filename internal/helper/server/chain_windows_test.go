//go:build windows

package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
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

func TestStartChainHandler_SysProxyAcceptsEmptyTunName(t *testing.T) {
	// Validation must accept sysproxy mode without a tun_name. The handler
	// will fail later (no real binaries on test box), but the validator gate
	// must pass.
	h := NewStartChainHandler()
	args := json.RawMessage(`{"singbox_config":{},"xray_config":{},"server_host":"x","server_port":1,"mode":"sysproxy"}`)
	_, err := h(context.Background(), args)
	if err == nil {
		return // unexpected success but not the failure we're testing
	}
	if strings.Contains(err.Error(), "tun_name required") {
		t.Fatalf("err=%v: validator should accept sysproxy without tun_name", err)
	}
}

type slowStopper struct {
	delay time.Duration
	err   error
}

func (s *slowStopper) Stop(_ time.Duration) error {
	time.Sleep(s.delay)
	return s.err
}

func TestStopBoth_RunsInParallel(t *testing.T) {
	a := &slowStopper{delay: 800 * time.Millisecond}
	b := &slowStopper{delay: 800 * time.Millisecond}
	start := time.Now()
	xerr, serr := stopBoth(2*time.Second, a, b)
	elapsed := time.Since(start)
	if xerr != nil || serr != nil {
		t.Fatalf("errs: %v %v", xerr, serr)
	}
	if elapsed >= 1500*time.Millisecond {
		t.Fatalf("elapsed=%v, want < 1.5s (parallel, not sequential)", elapsed)
	}
}

func TestStopBoth_NilSafe(t *testing.T) {
	xerr, serr := stopBoth(time.Second, nil, nil)
	if xerr != nil || serr != nil {
		t.Fatalf("nil cores: %v %v", xerr, serr)
	}
}

func TestStopBoth_PropagatesErrors(t *testing.T) {
	a := &slowStopper{err: errors.New("xray-fail")}
	b := &slowStopper{err: errors.New("sb-fail")}
	xerr, serr := stopBoth(time.Second, a, b)
	if xerr == nil || xerr.Error() != "xray-fail" {
		t.Fatalf("xerr=%v", xerr)
	}
	if serr == nil || serr.Error() != "sb-fail" {
		t.Fatalf("serr=%v", serr)
	}
}
