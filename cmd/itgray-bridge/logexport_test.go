package main

import (
	"os"
	"path/filepath"
	"testing"

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
