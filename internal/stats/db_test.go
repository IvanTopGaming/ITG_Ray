package stats

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpen_AppliesWALAndSynchronous(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.db")
	db, err := Open(p)
	require.NoError(t, err)
	defer db.Close()

	var mode string
	require.NoError(t, db.QueryRow("PRAGMA journal_mode").Scan(&mode))
	require.Equal(t, "wal", mode)

	var sync string
	require.NoError(t, db.QueryRow("PRAGMA synchronous").Scan(&sync))
	// 1 == NORMAL in SQLite
	require.Equal(t, "1", sync)
}

func TestOpen_CreatesSchema(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.db")
	db, err := Open(p)
	require.NoError(t, err)
	defer db.Close()

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	require.NoError(t, err)
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		names = append(names, n)
	}
	require.Contains(t, names, "connections")
	require.Contains(t, names, "hourly_rollup")
	require.Contains(t, names, "schema_version")
}
