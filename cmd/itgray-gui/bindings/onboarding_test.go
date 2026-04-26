package bindings

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOnboardingService_InitialState_False asserts a fresh DataDir reports
// onboarded=false: no marker file → first run.
func TestOnboardingService_InitialState_False(t *testing.T) {
	dir := t.TempDir()
	svc := NewOnboardingService(OnboardingDeps{DataDir: dir})
	got, err := svc.GetState(context.Background())
	require.NoError(t, err)
	require.Equal(t, map[string]any{"onboarded": false}, got)
}

// TestOnboardingService_Complete_WritesMarker verifies Complete writes the
// zero-byte .onboarded file at the configured path. Re-reading via
// GetState then reports onboarded=true.
func TestOnboardingService_Complete_WritesMarker(t *testing.T) {
	dir := t.TempDir()
	svc := NewOnboardingService(OnboardingDeps{DataDir: dir})
	require.NoError(t, svc.Complete(context.Background()))

	info, err := os.Stat(filepath.Join(dir, ".onboarded"))
	require.NoError(t, err)
	require.Equal(t, int64(0), info.Size(), "marker file must be zero bytes")

	got, err := svc.GetState(context.Background())
	require.NoError(t, err)
	require.Equal(t, map[string]any{"onboarded": true}, got)
}

// TestOnboardingService_Skip_WritesSameMarker verifies Skip behaves
// identically to Complete on disk — the wizard is dismissed either way.
func TestOnboardingService_Skip_WritesSameMarker(t *testing.T) {
	dir := t.TempDir()
	svc := NewOnboardingService(OnboardingDeps{DataDir: dir})
	require.NoError(t, svc.Skip(context.Background()))

	got, err := svc.GetState(context.Background())
	require.NoError(t, err)
	require.Equal(t, map[string]any{"onboarded": true}, got)
}
