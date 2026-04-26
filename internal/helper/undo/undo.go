// Package undo persists the Helper's pending state-changing operations so a
// crash leaves recoverable info under %ProgramData%/ITG Ray/Helper/undo.json.
package undo

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/itg-team/itg-ray/internal/helper/dns"
	"github.com/itg-team/itg-ray/internal/helper/route"
)

// Journal is one snapshot of all currently-pending Helper-side mutations.
type Journal struct {
	TunName  string         `json:"tun_name,omitempty"`
	Routes   []route.Entry  `json:"routes,omitempty"`
	DNSPrior []dns.Settings `json:"dns_prior,omitempty"`
}

// Load reads the journal at path; absent file is not an error and returns
// the zero value.
func Load(path string) (Journal, error) {
	b, err := os.ReadFile(path) //nolint:gosec // G304: path is supplied by Helper service code, not untrusted input
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Journal{}, nil
		}
		return Journal{}, err
	}
	var j Journal
	if err := json.Unmarshal(b, &j); err != nil {
		return Journal{}, err
	}
	return j, nil
}

// Save writes the journal atomically (tmp + rename).
func Save(path string, j Journal) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // G301: under %ProgramData% which is admin-only
		return err
	}
	b, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil { //nolint:gosec // G306: under %ProgramData% which is admin-only
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// Clear empties the journal — call after a clean shutdown.
func Clear(path string) error { return Save(path, Journal{}) }
