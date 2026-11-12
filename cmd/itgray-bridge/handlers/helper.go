package handlers

import (
	"context"
	"encoding/json"
	"time"
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
	InstallLinux() error
	UninstallLinux() error
	StartLinux() error
	StopLinux() error
	RestartLinux() error
}

// HelperHandlers groups methods under the "helper." namespace.
type HelperHandlers struct {
	Svc           Helper
	HelperVersion func(ctx context.Context) (string, error)
}

// helperStatus is the JSON-RPC result shape for helper.status. Mirrors
// protocol.HelperStatusResult; owned here to avoid an import cycle into
// the protocol/codegen package.
// helperIsPackageManaged is a var so tests can pin it — the real detector
// probes the host filesystem, which would otherwise make results depend on
// whether the machine running the tests has the distro package installed.
var helperIsPackageManaged = detectPackageManagedHelper

type helperStatus struct {
	State           string `json:"state"`
	PackageManaged  bool   `json:"packageManaged"`
	Version         string `json:"version,omitempty"`
	ExpectedVersion string `json:"expectedVersion,omitempty"`
}

const helperVersionTimeout = time.Second

// Status returns the helper service state ("running" / "stopped" / "missing").
// Nil-safe: returns {state:""} (no error) when Svc is unset.
func (h HelperHandlers) Status(ctx context.Context, _ json.RawMessage) (any, error) {
	managed := helperIsPackageManaged()
	if h.Svc == nil {
		return helperStatus{PackageManaged: managed}, nil
	}
	state, err := h.Svc.Status()
	if err != nil {
		return nil, err
	}
	out := helperStatus{State: state, PackageManaged: managed}
	if state == "running" && h.HelperVersion != nil {
		vctx, cancel := context.WithTimeout(ctx, helperVersionTimeout)
		v, verr := h.HelperVersion(vctx)
		cancel()
		if verr == nil && v != "" {
			out.Version = v
			out.ExpectedVersion = Version
		}
	}
	return out, nil
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

// InstallLinux installs the privileged systemd helper on Linux via pkexec
// (one polkit prompt). No-op-safe when Svc is unset.
func (h HelperHandlers) InstallLinux(_ context.Context, _ json.RawMessage) (any, error) {
	if h.Svc == nil {
		return struct{}{}, nil
	}
	return struct{}{}, h.Svc.InstallLinux()
}

// UninstallLinux removes the privileged systemd helper on Linux via pkexec
// (one polkit prompt). No-op-safe when Svc is unset.
func (h HelperHandlers) UninstallLinux(_ context.Context, _ json.RawMessage) (any, error) {
	if h.Svc == nil {
		return struct{}{}, nil
	}
	return struct{}{}, h.Svc.UninstallLinux()
}

func (h HelperHandlers) StartLinux(_ context.Context, _ json.RawMessage) (any, error) {
	if h.Svc == nil {
		return struct{}{}, nil
	}
	return struct{}{}, h.Svc.StartLinux()
}

func (h HelperHandlers) StopLinux(_ context.Context, _ json.RawMessage) (any, error) {
	if h.Svc == nil {
		return struct{}{}, nil
	}
	return struct{}{}, h.Svc.StopLinux()
}

func (h HelperHandlers) RestartLinux(_ context.Context, _ json.RawMessage) (any, error) {
	if h.Svc == nil {
		return struct{}{}, nil
	}
	return struct{}{}, h.Svc.RestartLinux()
}
