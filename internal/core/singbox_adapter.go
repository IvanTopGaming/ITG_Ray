// Package core owns the lifecycle of the embedded sing-box and xray instances.
package core

// Tested against sing-box v1.13.11
//
// API notes (v1.13.11):
//   - sb.New takes sb.Options (not a pointer); sb.Options embeds sbopt.Options and adds Context.
//   - sbopt.Options does NOT implement encoding/json.Unmarshaler; use UnmarshalJSONContext
//     which runs sing-box's context-aware decoder (required for type-tagged fields).
//   - box.Start() is a one-shot call; box.PreStart() is an optional pre-flight check
//     (not used here).

import (
	"context"
	"fmt"

	sb "github.com/sagernet/sing-box"
	sbopt "github.com/sagernet/sing-box/option"
)

// SingboxAdapter wraps the embedded sing-box instance lifecycle.
type SingboxAdapter struct {
	inst *sb.Box
}

// NewSingboxAdapter returns an idle adapter ready for Start.
func NewSingboxAdapter() *SingboxAdapter { return &SingboxAdapter{} }

// Start unmarshals configJSON into sing-box option.Options, creates and starts
// the embedded sing-box instance. Returns an error if the instance is already
// running or if any step fails.
func (a *SingboxAdapter) Start(ctx context.Context, configJSON []byte) error {
	if a.inst != nil {
		return fmt.Errorf("sing-box already running")
	}

	var opts sbopt.Options
	// UnmarshalJSONContext is required: it uses sing-box's context-aware JSON
	// decoder that resolves type-tagged fields (inbounds, outbounds, …).
	if err := opts.UnmarshalJSONContext(ctx, configJSON); err != nil {
		return fmt.Errorf("sing-box options unmarshal: %w", err)
	}

	box, err := sb.New(sb.Options{
		Context: ctx,
		Options: opts,
	})
	if err != nil {
		return fmt.Errorf("sing-box new: %w", err)
	}

	if err := box.Start(); err != nil {
		return fmt.Errorf("sing-box start: %w", err)
	}

	a.inst = box
	return nil
}

// Close stops the running sing-box instance and resets the adapter so it may
// be reused. Returns nil if no instance is running.
func (a *SingboxAdapter) Close() error {
	if a.inst == nil {
		return nil
	}
	err := a.inst.Close()
	a.inst = nil
	return err
}
