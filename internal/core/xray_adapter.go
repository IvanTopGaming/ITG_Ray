// Package core owns the lifecycle of the embedded sing-box and xray instances.
package core

// Tested against xray-core v1.260327.0
//
// API notes (v1.260327.0):
//   - xserial.LoadJSONConfig(io.Reader) (*xcore.Config, error) — parses the
//     JSON config and bridges it to the protobuf-encoded *core.Config.
//   - xcore.New(*Config) (*Instance, error) — constructs but does not start.
//   - inst.Start() / inst.Close() — one-shot lifecycle methods.
//   - No CGO dependencies: xray-core v1.260327.0 is pure-Go; cross-compilation
//     to Windows (GOOS=windows GOARCH=amd64) is unaffected.

import (
	"bytes"
	"context"
	"fmt"

	xcore "github.com/xtls/xray-core/core"
	xserial "github.com/xtls/xray-core/infra/conf/serial"
)

// XrayAdapter wraps an embedded xray-core instance.
type XrayAdapter struct {
	inst *xcore.Instance
}

// NewXrayAdapter returns an unstarted XrayAdapter.
func NewXrayAdapter() *XrayAdapter { return &XrayAdapter{} }

// Start parses the xray JSON config and starts the embedded instance.
// Returns an error if the instance is already running or any step fails.
func (a *XrayAdapter) Start(_ context.Context, configJSON []byte) error {
	if a.inst != nil {
		return fmt.Errorf("xray already running")
	}
	cfg, err := xserial.LoadJSONConfig(bytes.NewReader(configJSON))
	if err != nil {
		return fmt.Errorf("xray load json: %w", err)
	}
	inst, err := xcore.New(cfg)
	if err != nil {
		return fmt.Errorf("xray new: %w", err)
	}
	if err := inst.Start(); err != nil {
		return fmt.Errorf("xray start: %w", err)
	}
	a.inst = inst
	return nil
}

// Close stops the running xray-core instance and resets the adapter so it may
// be reused. Returns nil if no instance is running.
func (a *XrayAdapter) Close() error {
	if a.inst == nil {
		return nil
	}
	err := a.inst.Close()
	a.inst = nil
	return err
}
