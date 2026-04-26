package subscription

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/itg-team/itg-ray/internal/config"
)

// updateMetaLocks serializes UpdateMeta load-mutate-save sequences across all
// FileStore instances that share the same Path. Keyed by the cleaned absolute
// path so two FileStore values addressing the same file cannot interleave.
var (
	updateMetaLocksMu sync.Mutex
	updateMetaLocks   = map[string]*sync.Mutex{}
)

func lockForPath(path string) *sync.Mutex {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	abs = filepath.Clean(abs)
	updateMetaLocksMu.Lock()
	defer updateMetaLocksMu.Unlock()
	m, ok := updateMetaLocks[abs]
	if !ok {
		m = &sync.Mutex{}
		updateMetaLocks[abs] = m
	}
	return m
}

// Stored is the on-disk shape of one subscription entry. It includes the
// metadata fields populated by refresh.Driver: LastSyncAt and LastStatus
// are written after every sync attempt (success or failure).
//
// Note: omitempty has no effect on time.Time (it's a struct), so a fresh
// subscription that has never been synced will round-trip with
// LastSyncAt = "0001-01-01T00:00:00Z" in JSON. Use LastSyncAt.IsZero() to
// distinguish "never synced" from a real timestamp.
type Stored struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	URL            string    `json:"url"`
	UserAgent      string    `json:"user_agent,omitempty"`
	UpdateInterval Duration  `json:"update_interval,omitempty"`
	LastSyncAt     time.Time `json:"last_sync_at,omitempty"`
	LastStatus     string    `json:"last_status,omitempty"`
}

// ToSyncInput converts a Stored entry into the in-memory Subscription type
// consumed by Sync(). Auth defaults to AuthNone() because Sync calls
// sub.Auth(req) unconditionally.
func (s Stored) ToSyncInput() Subscription {
	return Subscription{
		ID:             s.ID,
		Name:           s.Name,
		URL:            s.URL,
		UserAgent:      s.UserAgent,
		Auth:           AuthNone(),
		UpdateInterval: time.Duration(s.UpdateInterval),
	}
}

// Store is the persistence interface consumed by refresh.Driver and the CLI.
type Store interface {
	Load() ([]Stored, error)
	Save(subs []Stored) error
	UpdateMeta(id string, at time.Time, status string) error
}

// FileStore persists Stored entries as JSON at Path, using atomic file replace.
type FileStore struct {
	Path string
}

type subsFileEnvelope struct {
	Subs []Stored `json:"subs"`
}

// Load reads the file and returns the subs. A missing file is treated as
// "no subscriptions" (empty slice, nil error).
func (s FileStore) Load() ([]Stored, error) {
	b, err := os.ReadFile(s.Path) //nolint:gosec // path is application-controlled
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", s.Path, err)
	}
	var env subsFileEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", s.Path, err)
	}
	return env.Subs, nil
}

// Save replaces the file atomically. Empty slice is allowed (file becomes {"subs":[]}).
func (s FileStore) Save(subs []Stored) error {
	if subs == nil {
		subs = []Stored{}
	}
	b, err := json.MarshalIndent(subsFileEnvelope{Subs: subs}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return config.WriteAtomic(s.Path, b, 0o600)
}

// UpdateMeta mutates LastSyncAt + LastStatus for one ID and saves. Unknown
// IDs are a no-op (no error) — the user may have removed the sub in a race.
func (s FileStore) UpdateMeta(id string, at time.Time, status string) error {
	mu := lockForPath(s.Path)
	mu.Lock()
	defer mu.Unlock()
	subs, err := s.Load()
	if err != nil {
		return err
	}
	for i := range subs {
		if subs[i].ID == id {
			subs[i].LastSyncAt = at
			subs[i].LastStatus = status
			return s.Save(subs)
		}
	}
	return nil
}
