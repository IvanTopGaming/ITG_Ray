package bindings

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// onboardedMarker is the zero-byte file written into DataDir to mark the
// first-run wizard as complete. Mirrors AppService.isOnboarded which
// stat()s the same path; keeping the constant here avoids a package-level
// re-export.
const onboardedMarker = ".onboarded"

// OnboardingDeps groups the dependencies passed in from main.go. DataDir
// is the per-user config root that AppService and SubsService also write
// into, so the marker lives next to servers.json / subscriptions.json.
type OnboardingDeps struct {
	DataDir string
}

// OnboardingService implements the Onboarding.* Wails bindings used by
// the first-run wizard: GetState reports whether the marker exists,
// Skip and Complete both write the marker (zero-byte file).
type OnboardingService struct {
	d OnboardingDeps
}

// NewOnboardingService constructs a new OnboardingService.
func NewOnboardingService(d OnboardingDeps) *OnboardingService {
	return &OnboardingService{d: d}
}

// GetState returns {"onboarded": true} when the marker file exists. Any
// stat error other than os.IsNotExist would indicate a permissions or
// disk problem; we surface those so the wizard does not silently treat
// a broken DataDir as "first run".
func (o *OnboardingService) GetState(_ context.Context) (map[string]any, error) {
	_, err := os.Stat(o.markerPath())
	if err == nil {
		return map[string]any{"onboarded": true}, nil
	}
	if os.IsNotExist(err) {
		return map[string]any{"onboarded": false}, nil
	}
	return nil, fmt.Errorf("stat onboarded marker: %w", err)
}

// Complete writes the marker file. Idempotent — calling twice is safe.
func (o *OnboardingService) Complete(_ context.Context) error { return o.mark() }

// Skip writes the same marker as Complete so users who dismiss the
// wizard are not nagged on the next launch. The frontend records the
// difference (Skip vs Complete) only in telemetry; the on-disk state
// is identical.
func (o *OnboardingService) Skip(_ context.Context) error { return o.mark() }

func (o *OnboardingService) markerPath() string {
	return filepath.Join(o.d.DataDir, onboardedMarker)
}

func (o *OnboardingService) mark() error {
	if err := os.MkdirAll(o.d.DataDir, 0o750); err != nil {
		return fmt.Errorf("mkdir data dir: %w", err)
	}
	f, err := os.Create(o.markerPath())
	if err != nil {
		return fmt.Errorf("create onboarded marker: %w", err)
	}
	return f.Close()
}
