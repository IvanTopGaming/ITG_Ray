package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeOnboarder struct {
	state    map[string]any
	stateErr error
	calls    []string
}

func (f *fakeOnboarder) GetState() (map[string]any, error) {
	if f.stateErr != nil {
		return nil, f.stateErr
	}
	if f.state == nil {
		return map[string]any{"onboarded": false}, nil
	}
	return f.state, nil
}
func (f *fakeOnboarder) Complete() error { f.calls = append(f.calls, "complete"); return nil }
func (f *fakeOnboarder) Skip() error     { f.calls = append(f.calls, "skip"); return nil }

func TestOnboardingGetStateReturnsOnboardedBool(t *testing.T) {
	tests := []struct {
		name     string
		fake     *fakeOnboarder
		wantJSON string
	}{
		{
			name:     "onboarded",
			fake:     &fakeOnboarder{state: map[string]any{"onboarded": true}},
			wantJSON: `{"onboarded":true}`,
		},
		{
			name:     "not onboarded",
			fake:     &fakeOnboarder{state: map[string]any{"onboarded": false}},
			wantJSON: `{"onboarded":false}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := OnboardingHandlers{Svc: tt.fake}
			result, err := h.GetState(context.Background(), nil)
			if err != nil {
				t.Fatalf("GetState: %v", err)
			}
			raw, _ := json.Marshal(result)
			if string(raw) != tt.wantJSON {
				t.Fatalf("got=%s want=%s", raw, tt.wantJSON)
			}
		})
	}
}

func TestOnboardingGetStatePropagatesError(t *testing.T) {
	h := OnboardingHandlers{Svc: &fakeOnboarder{stateErr: errors.New("disk full")}}
	if _, err := h.GetState(context.Background(), nil); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestOnboardingCompleteAndSkip(t *testing.T) {
	fake := &fakeOnboarder{}
	h := OnboardingHandlers{Svc: fake}
	if _, err := h.Complete(context.Background(), nil); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if _, err := h.Skip(context.Background(), nil); err != nil {
		t.Fatalf("Skip: %v", err)
	}
	if len(fake.calls) != 2 || fake.calls[0] != "complete" || fake.calls[1] != "skip" {
		t.Fatalf("unexpected call sequence: %v", fake.calls)
	}
}
