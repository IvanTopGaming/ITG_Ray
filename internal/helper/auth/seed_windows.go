//go:build windows

package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Seed writes the SID into AllowedSIDFile, replacing prior contents.
func Seed(sid string) error {
	//nolint:gosec // G301: ProgramData\ITG Ray\Helper holds a public allow-list — diagnostic tools need world-read
	if err := os.MkdirAll(filepath.Dir(AllowedSIDFile), 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf("# ITG Ray helper allow-list — managed by itgray-cli helper install\n%s\n", strings.TrimSpace(sid))
	//nolint:gosec // SID list is a public allow-list — read access is intended for diagnostic tools
	return os.WriteFile(AllowedSIDFile, []byte(content), 0o644)
}

// Load reads and parses AllowedSIDFile.
func Load() ([]string, error) {
	b, err := os.ReadFile(AllowedSIDFile)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, nil
}
