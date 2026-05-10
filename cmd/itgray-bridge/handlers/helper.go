package handlers

import (
	"context"
	"encoding/json"
)

// Helper is the surface HelperHandlers needs from bindings.HelperService.
// The real type satisfies it directly. Linux/macOS bindings.HelperService
// methods return errors via svcmgr's cross-platform stubs.
type Helper interface {
	Status() (string, error)
	Install(exePath string) error
	Start() error
	Stop() error
	Restart() error
	Reinstall() error
}

// HelperHandlers groups methods under the "helper." namespace.
type HelperHandlers struct {
	Svc Helper
}

// helperStatus is the JSON-RPC result shape for helper.status. Mirrors
// protocol.HelperStatusResult; owned here to avoid an import cycle into
// the protocol/codegen package.
type helperStatus struct {
	State string `json:"state"`
}

// Status returns the helper service state ("running" / "stopped" / "missing").
// Nil-safe: returns {state:""} (no error) when Svc is unset.
func (h HelperHandlers) Status(_ context.Context, _ json.RawMessage) (any, error) {
	if h.Svc == nil {
		return helperStatus{}, nil
	}
	state, err := h.Svc.Status()
	if err != nil {
		return nil, err
	}
	return helperStatus{State: state}, nil
}

// Install registers the helper service. The bridge always passes an empty
// exePath so the bindings layer falls back to defaultHelperExePath()
// (helper binary colocated with the running process).
func (h HelperHandlers) Install(_ context.Context, _ json.RawMessage) (any, error) {
	if h.Svc == nil {
		return struct{}{}, nil
	}
	return struct{}{}, h.Svc.Install("")
}

// Start asks SCM to start the helper. UAC prompt on Windows.
func (h HelperHandlers) Start(_ context.Context, _ json.RawMessage) (any, error) {
	if h.Svc == nil {
		return struct{}{}, nil
	}
	return struct{}{}, h.Svc.Start()
}

// Stop asks SCM to stop the helper. UAC prompt on Windows.
func (h HelperHandlers) Stop(_ context.Context, _ json.RawMessage) (any, error) {
	if h.Svc == nil {
		return struct{}{}, nil
	}
	return struct{}{}, h.Svc.Stop()
}

// Restart stop+start in one elevated call (one UAC prompt).
func (h HelperHandlers) Restart(_ context.Context, _ json.RawMessage) (any, error) {
	if h.Svc == nil {
		return struct{}{}, nil
	}
	return struct{}{}, h.Svc.Restart()
}

// Reinstall stops, removes, re-registers, and starts the helper in one
// elevated call (one UAC prompt).
func (h HelperHandlers) Reinstall(_ context.Context, _ json.RawMessage) (any, error) {
	if h.Svc == nil {
		return struct{}{}, nil
	}
	return struct{}{}, h.Svc.Reinstall()
}
