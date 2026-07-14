package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/itg-team/itg-ray/internal/logstream"
	"github.com/stretchr/testify/require"
)

func TestReadCombinedLogs_OldestFirst(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "app.log")
	require.NoError(t, os.WriteFile(p+".2", []byte("oldest\n"), 0o640))
	require.NoError(t, os.WriteFile(p+".1", []byte("middle\n"), 0o640))
	require.NoError(t, os.WriteFile(p, []byte("newest\n"), 0o640))
	out := readCombinedLogs(p, 3)
	require.Equal(t, "oldest\nmiddle\nnewest\n", out)
}

func TestReadCombinedLogs_MissingFilesSkipped(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "app.log")
	require.NoError(t, os.WriteFile(p, []byte("only\n"), 0o640))
	require.Equal(t, "only\n", readCombinedLogs(p, 3))
}

func TestCombinedExport_AppendsNonBridgeBufferEntries(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "app.log")
	require.NoError(t, os.WriteFile(p+".1", []byte("bridge-rotated\n"), 0o640))
	require.NoError(t, os.WriteFile(p, []byte("bridge-current line-marker\n"), 0o640))

	ts := time.Date(2026, 7, 14, 3, 0, 0, 0, time.UTC)
	entries := []logstream.Entry{
		{Seq: 1, Time: ts, Level: "ERROR", Source: "helper", Message: "helper-firewall-fail"},
		{Seq: 2, Time: ts, Level: "INFO", Source: "sing-box", Message: "singbox-route-up"},
		{Seq: 3, Time: ts, Level: "INFO", Source: "bridge", Message: "line-marker"},
	}
	out := combinedExport(p, 3, entries)

	require.Contains(t, out, "bridge-rotated")
	require.Contains(t, out, "bridge-current line-marker")
	require.Contains(t, out, "ERROR [helper] helper-firewall-fail")
	require.Contains(t, out, "INFO [sing-box] singbox-route-up")
	require.NotContains(t, out, "[bridge] line-marker")
}
