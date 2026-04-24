package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Load reads a servers.json file. Returns an empty slice (not nil-or-error)
// when the file does not yet exist.
func Load(path string) ([]Server, error) {
	b, err := os.ReadFile(path) //nolint:gosec // path is application-controlled, not user input
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []Server
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Save writes servers atomically (tmp + rename) with 0600 permissions.
func Save(path string, servers []Server) error {
	b, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// Merge reconciles an existing server list with a freshly-synced list for a given
// subscription source. Existing-server local fields (Tags, LatencyMS, Favorite, Disabled)
// are preserved across sync by matching on stable ID. Existing entries whose origin is
// OriginSubscription with matching sourceID but missing from the incoming list are dropped;
// entries from other sources are preserved untouched.
func Merge(existing, incoming []Server, sourceID string) []Server {
	existingByID := make(map[string]Server, len(existing))
	for i := range existing {
		existingByID[existing[i].ID] = existing[i]
	}

	var out []Server
	seen := make(map[string]bool)

	for i := range incoming {
		s := incoming[i]
		seen[s.ID] = true
		if old, ok := existingByID[s.ID]; ok {
			s.Tags = old.Tags
			s.LatencyMS = old.LatencyMS
			s.Favorite = old.Favorite
			s.Disabled = old.Disabled
		}
		out = append(out, s)
	}
	for i := range existing {
		old := existing[i]
		if seen[old.ID] {
			continue
		}
		if old.Origin == OriginSubscription && old.SourceID == sourceID {
			continue // this sync deleted it
		}
		out = append(out, old)
	}
	return out
}
