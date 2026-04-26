package bindings

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"

	"github.com/stretchr/testify/require"
)

// newSettingsServiceForTest builds a SettingsService over a fresh
// ConfigStore rooted in dir. The version/buildDate strings are fixed so
// the About projection is deterministic.
func newSettingsServiceForTest(t *testing.T, dir string) *SettingsService {
	t.Helper()
	store := NewConfigStore(filepath.Join(dir, "config.json"), "test", "2026-04-26")
	return NewSettingsService(SettingsDeps{Store: store, Hub: hub.New()})
}

// TestSettingsService_Get_DefaultsWhenMissing verifies Get returns the
// internal/config defaults when config.json does not yet exist on disk.
// A first-run install must not hand the frontend an empty view.
func TestSettingsService_Get_DefaultsWhenMissing(t *testing.T) {
	svc := newSettingsServiceForTest(t, t.TempDir())

	view, err := svc.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "en", view.General.Language)
	require.Equal(t, "dark", view.General.Theme)
	require.True(t, view.General.CloseToTray)
	require.Equal(t, "tun", view.Network.DefaultMode)
	require.Equal(t, "198.18.0.1/15", view.Network.TunCIDR)
	require.Equal(t, "test", view.About.Version)
	// Security detection is a v0.2 follow-up — see configstore.go.
	require.Equal(t, "Unknown", view.Security.Method)
}

// TestSettingsService_Update_PersistsAcrossGet writes a general patch and
// verifies a subsequent Get returns the updated value, exercising the
// load-mutate-save round-trip through internal/config.
func TestSettingsService_Update_PersistsAcrossGet(t *testing.T) {
	svc := newSettingsServiceForTest(t, t.TempDir())

	view, err := svc.Update(context.Background(), "general", map[string]any{"language": "ru"})
	require.NoError(t, err)
	require.Equal(t, "ru", view.General.Language)

	view2, err := svc.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "ru", view2.General.Language)
}

// TestSettingsService_Update_UnknownSectionErrors guards the section name
// allowlist: typos must surface as binding errors rather than silently
// no-op-ing on disk.
func TestSettingsService_Update_UnknownSectionErrors(t *testing.T) {
	svc := newSettingsServiceForTest(t, t.TempDir())

	_, err := svc.Update(context.Background(), "made-up-section", map[string]any{"x": 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown section")
}

// TestSettingsService_Update_ConcurrentSafe runs 8 goroutines hammering
// Update on the same SettingsService. The path-keyed mutex inside
// ConfigStore must serialise the load-mutate-save cycle so the final
// on-disk state is one of the legal patches — not a torn merge. We do not
// assert which goroutine wins; only that no error fires and the final
// language is one of the values written.
func TestSettingsService_Update_ConcurrentSafe(t *testing.T) {
	svc := newSettingsServiceForTest(t, t.TempDir())

	const goroutines = 8
	langs := []string{"en", "ru", "auto"}

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		lang := langs[i%len(langs)]
		go func(l string) {
			defer wg.Done()
			_, err := svc.Update(context.Background(), "general", map[string]any{"language": l})
			require.NoError(t, err)
		}(lang)
	}
	wg.Wait()

	view, err := svc.Get(context.Background())
	require.NoError(t, err)
	require.Contains(t, langs, view.General.Language)
}
