package handlers

import (
	"context"
	"encoding/json"
)

// Onboarder is the surface OnboardingHandlers needs from
// bindings.OnboardingService. The real type satisfies it directly.
type Onboarder interface {
	GetState() (map[string]any, error)
	Complete() error
	Skip() error
}

// OnboardingHandlers groups methods under the "onboarding." namespace.
type OnboardingHandlers struct {
	Svc Onboarder
}

// onboardingState is the JSON-RPC result shape for onboarding.getState.
// Mirrors protocol.OnboardingStateResult (single bool) but is owned here
// to avoid an import cycle into the protocol/codegen package.
type onboardingState struct {
	Onboarded bool `json:"onboarded"`
}

// GetState returns whether the first-run wizard has been completed.
// The underlying bindings.OnboardingService returns map[string]any with
// an "onboarded" bool key — this handler narrows that to the typed shape.
func (o OnboardingHandlers) GetState(_ context.Context, _ json.RawMessage) (any, error) {
	if o.Svc == nil {
		return onboardingState{Onboarded: false}, nil
	}
	m, err := o.Svc.GetState()
	if err != nil {
		return nil, err
	}
	v, _ := m["onboarded"].(bool)
	return onboardingState{Onboarded: v}, nil
}

// Complete writes the onboarded marker. Idempotent.
func (o OnboardingHandlers) Complete(_ context.Context, _ json.RawMessage) (any, error) {
	if o.Svc == nil {
		return struct{}{}, nil
	}
	return struct{}{}, o.Svc.Complete()
}

// Skip is functionally identical to Complete (writes the same marker).
// The frontend records the difference (Skip vs Complete) only in
// telemetry; on-disk state is the same.
func (o OnboardingHandlers) Skip(_ context.Context, _ json.RawMessage) (any, error) {
	if o.Svc == nil {
		return struct{}{}, nil
	}
	return struct{}{}, o.Svc.Skip()
}
