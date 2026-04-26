// Package runtime is the source of truth for the Helper's per-session
// scratch directory layout. All child-process configs and log files
// live here, under %ProgramData%\ITG Ray\Helper\runtime\.
//
// The base directory is overridable via the ITGRAY_RUNTIME_BASE env var
// for tests; in production it is derived from %ProgramData%.
package runtime

import (
	"os"
	"path/filepath"
	"strings"
)

// BasePath returns the absolute path of the runtime directory.
func BasePath() string {
	if override := os.Getenv("ITGRAY_RUNTIME_BASE"); override != "" {
		return override
	}
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	return filepath.Join(pd, "ITG Ray", "Helper", "runtime")
}

// ConfigPath returns BasePath() + name for a config file.
func ConfigPath(name string) string { return filepath.Join(BasePath(), name) }

// LogPath returns BasePath() + name for a log file.
func LogPath(name string) string { return filepath.Join(BasePath(), name) }

// isLogFile reports whether name is a log artifact that EnsureClean should
// preserve so RotateLog can manage them across sessions.
func isLogFile(name string) bool {
	if strings.HasSuffix(name, ".log") {
		return true
	}
	// Match .log.1, .log.2, etc.
	if idx := strings.Index(name, ".log."); idx >= 0 {
		return true
	}
	return false
}

// EnsureClean creates BasePath() (with admin-only perms) and wipes any
// stale files left over from a prior session. Call at the top of
// OpStartChain. Log files (*.log, *.log.N) are preserved so RotateLog
// has prior sessions' logs to rotate.
func EnsureClean() error {
	base := BasePath()
	if err := os.MkdirAll(base, 0o750); err != nil { //nolint:gosec // %ProgramData%, admin-only
		return err
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if isLogFile(e.Name()) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(base, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
