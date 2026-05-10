package chainctl

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/itg-team/itg-ray/internal/config"
)

// sessionRecord is the on-disk last-session.json shape. Persisted on
// every successful Connect; cleared on Stop. Used by Reconcile() to
// rebind an already-running helper chain on app boot, and by tray
// "Connect (last server)" actions.
type sessionRecord struct {
	ServerID string    `json:"serverId"`
	Mode     string    `json:"mode"`
	At       time.Time `json:"at"`
}

// sessionPath returns the absolute path to last-session.json under the
// configured data directory.
func sessionPath(dataDir string) string {
	return filepath.Join(dataDir, "last-session.json")
}

// saveSession writes the record atomically (tmp + rename, 0600).
func saveSession(dataDir string, r sessionRecord) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return config.WriteAtomic(sessionPath(dataDir), b, 0o600)
}

// loadSession reads the record. A missing file returns a zero-value
// record and a nil error — callers treat empty ServerID as "no prior
// session".
func loadSession(dataDir string) (sessionRecord, error) {
	b, err := os.ReadFile(sessionPath(dataDir)) //nolint:gosec // path is application-controlled
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return sessionRecord{}, nil
		}
		return sessionRecord{}, err
	}
	var r sessionRecord
	if err := json.Unmarshal(b, &r); err != nil {
		return sessionRecord{}, err
	}
	return r, nil
}

// clearSession removes the on-disk record. Missing file is not an
// error.
func clearSession(dataDir string) error {
	if err := os.Remove(sessionPath(dataDir)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
