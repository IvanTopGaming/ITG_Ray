package bindings

import (
	"github.com/itg-team/itg-ray/cmd/itgray-gui/hub"
)

// SettingsConfigStore is the read/write surface SettingsService needs.
// ConfigStore (this package) is the only production implementation; tests
// can swap in fakes without touching disk. View must be safe for
// concurrent calls; UpdateSection serialises its own load-mutate-save.
type SettingsConfigStore interface {
	View() (hub.SettingsView, error)
	UpdateSection(section string, patch map[string]any) (hub.SettingsView, error)
}

// SettingsDeps groups dependencies passed in from main.go.
type SettingsDeps struct {
	Store SettingsConfigStore
	Hub   *hub.Hub
}

// SettingsService implements the Settings.* Wails bindings shipped in
// C.T12: Get returns the full SettingsView, Update merges a per-section
// patch and emits hub.EventSettings.
type SettingsService struct{ d SettingsDeps }

// NewSettingsService constructs a new SettingsService.
func NewSettingsService(d SettingsDeps) *SettingsService {
	return &SettingsService{d: d}
}

// Get returns the current SettingsView. Errors come from the on-disk
// config loader (file unreadable / corrupt JSON). A missing config.json
// is *not* an error — internal/config.Load returns defaults.
func (s *SettingsService) Get() (hub.SettingsView, error) {
	return s.d.Store.View()
}

// Update merges patch into the named section, persists atomically, and
// publishes hub.EventSettings so subscribers (the frontend store reducer,
// the tray) can refresh without a full snapshot reload. Returns the
// post-merge SettingsView so the caller can update its local state
// without a follow-up Get.
func (s *SettingsService) Update(section string, patch map[string]any) (hub.SettingsView, error) {
	view, err := s.d.Store.UpdateSection(section, patch)
	if err != nil {
		return hub.SettingsView{}, err
	}
	if s.d.Hub != nil {
		s.d.Hub.Publish(hub.Event{
			Name:    hub.EventSettings,
			Payload: map[string]any{"section": section},
		})
	}
	return view, nil
}
