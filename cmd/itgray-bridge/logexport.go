package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/itg-team/itg-ray/internal/logstream"
)

// readCombinedLogs concatenates the rotated log history oldest-first:
// path.<keep> … path.1 then the current path. Missing files are skipped.
func readCombinedLogs(path string, keep int) string {
	var b strings.Builder
	for i := keep; i >= 1; i-- {
		if data, err := os.ReadFile(fmt.Sprintf("%s.%d", path, i)); err == nil {
			b.Write(data)
		}
	}
	if data, err := os.ReadFile(path); err == nil { //nolint:gosec // path is app-controlled, not attacker-supplied
		b.Write(data)
	}
	return b.String()
}

// combinedExport returns the on-disk bridge history (readCombinedLogs) followed
// by the non-bridge in-memory buffer entries (helper + engine lines the poller
// injects directly into the ring, which never pass through slog and so are
// absent from app.log). Bridge entries are skipped to avoid duplicating the
// history already read from disk.
func combinedExport(path string, keep int, entries []logstream.Entry) string {
	var b strings.Builder
	b.WriteString(readCombinedLogs(path, keep))
	for _, e := range entries {
		if e.Source == "bridge" {
			continue
		}
		b.WriteString(e.Time.Format("2006-01-02T15:04:05.000Z07:00"))
		b.WriteByte(' ')
		b.WriteString(e.Level)
		b.WriteString(" [")
		b.WriteString(e.Source)
		b.WriteString("] ")
		b.WriteString(e.Message)
		b.WriteByte('\n')
	}
	return b.String()
}
