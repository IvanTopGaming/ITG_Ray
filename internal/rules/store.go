package rules

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/itg-team/itg-ray/internal/config"
)

// Store owns the on-disk rules.json for the user-config data dir. Load
// returns the canonical Safety + My Rules default when the file is
// missing or corrupt — a corrupt rules.json must never block Connect.
// Save writes atomically (tmp + rename) at 0o600 so readers never see
// a torn document.
type Store struct {
	mu   sync.Mutex
	path string
}

// NewStore returns a Store bound to <dataDir>/rules.json.
func NewStore(dataDir string) *Store {
	return &Store{path: filepath.Join(dataDir, "rules.json")}
}

// Path returns the absolute file path the store reads and writes.
func (s *Store) Path() string { return s.path }

// Load reads rules.json or returns the default model when the file is
// missing or unparsable. Atomic with respect to concurrent Save callers.
func (s *Store) Load() (Model, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path) //nolint:gosec // path is application-controlled
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return DefaultModel(), nil
		}
		return DefaultModel(), nil
	}
	var m Model
	if err := json.Unmarshal(b, &m); err != nil {
		return DefaultModel(), nil
	}
	return m, nil
}

// Save writes m atomically to rules.json. Holds the store mutex so two
// concurrent Save callers see a consistent on-disk sequence.
func (s *Store) Save(m Model) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return config.WriteAtomic(s.path, b, 0o600)
}

// DefaultModel returns the Safety + My Rules layout used when no
// rules.json exists yet. Mirrors the historical loadRulesFromDataDir
// behavior so first-time users keep working LAN-bypass out of the box.
func DefaultModel() Model {
	return Model{
		DefaultAction: ActionProxy,
		Groups: []Group{
			{
				ID:      "safety",
				Name:    "Safety",
				Locked:  true,
				Enabled: true,
				Rules: []Rule{{
					ID:      "private",
					Name:    "Private IPs",
					Enabled: true,
					Action:  ActionDirect,
					Conditions: Conditions{IPCIDRs: []string{
						"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
						"127.0.0.0/8", "fc00::/7", "fe80::/10", "224.0.0.0/4",
					}},
				}},
			},
			{ID: "user", Name: "My Rules", Enabled: true},
		},
	}
}
