// Package hwid resolves a stable per-installation device identifier for
// subscription-fetch headers (Remnawave x-hwid contract). The value is
// HMAC-SHA-256 over the platform machine-id, hex-encoded, salted with a
// fixed app salt so the same hardware produces a different ID per app.
package hwid

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/denisbrodbeck/machineid"
	"github.com/itg-team/itg-ray/internal/config"
)

const (
	appSalt        = "itg-ray"
	cacheFilename  = "hwid.dat"
	expectedHexLen = 64
)

// Get returns the cached HWID hex string, computing and persisting it on
// first call. configDir is the per-user config root (same dir as
// subscriptions.json).
//
// Resolution order:
//  1. read configDir/hwid.dat — if exists and is valid 64-char hex, return it
//  2. compute machineid.ProtectedID(appSalt) — if succeeds, persist + return
//  3. fallback: 32 random bytes via crypto/rand → SHA-256 → hex; persist + return
//
// Step 3 ensures we always return SOMETHING usable. Persisted value lives
// forever unless the user deletes hwid.dat manually.
//
// Returns a non-nil error in two non-fatal cases:
//   - cache write failed (value still usable, will be recomputed next call)
//   - machineid was unavailable (random fallback used; non-deterministic across
//     re-installs). Callers should log the error but use the returned value.
func Get(configDir string) (string, error) {
	cachePath := filepath.Join(configDir, cacheFilename)
	if v, err := readCache(cachePath); err == nil {
		return v, nil
	}

	v, midErr := machineid.ProtectedID(appSalt)
	if midErr != nil {
		v = randomHex()
	}
	if writeErr := writeCache(cachePath, v); writeErr != nil {
		return v, fmt.Errorf("hwid cache write: %w", writeErr)
	}
	if midErr != nil {
		return v, fmt.Errorf("hwid machineid unavailable, used random fallback: %w", midErr)
	}
	return v, nil
}

func readCache(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := string(b)
	if len(s) != expectedHexLen {
		return "", errors.New("invalid length")
	}
	if _, err := hex.DecodeString(s); err != nil {
		return "", err
	}
	return s, nil
}

func writeCache(path, v string) error {
	return config.WriteAtomic(path, []byte(v), 0o600)
}

func randomHex() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand documented as never-fail in practice
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
