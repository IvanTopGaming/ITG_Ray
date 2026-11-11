package clashapi

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const secretFile = "clash-secret"

func LoadOrCreateSecret(dataDir string) (string, error) {
	path := filepath.Join(dataDir, secretFile)
	if b, err := os.ReadFile(path); err == nil { //nolint:gosec // path is the app data dir
		if s := strings.TrimSpace(string(b)); len(s) == 64 {
			if _, err := hex.DecodeString(s); err == nil {
				return s, nil
			}
		}
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("clash secret: %w", err)
	}
	s := hex.EncodeToString(buf)
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return "", fmt.Errorf("clash secret: %w", err)
	}
	if err := os.WriteFile(path, []byte(s), 0o600); err != nil {
		return "", fmt.Errorf("clash secret: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", fmt.Errorf("clash secret: %w", err)
	}
	return s, nil
}
