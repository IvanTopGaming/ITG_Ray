// Package stats persists hourly traffic rollups (no destinations) to SQLite.
package stats

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const schemaVersion = 2

const schema = `
CREATE TABLE IF NOT EXISTS schema_version (v INTEGER NOT NULL);
DROP TABLE IF EXISTS connections;
DROP TABLE IF EXISTS hourly_rollup;
CREATE TABLE IF NOT EXISTS rollup (
	hour_utc   INTEGER NOT NULL,
	app        TEXT    NOT NULL,
	rule_id    TEXT    NOT NULL,
	route      TEXT    NOT NULL,
	server_id  TEXT    NOT NULL,
	bytes_up   INTEGER NOT NULL DEFAULT 0,
	bytes_down INTEGER NOT NULL DEFAULT 0,
	conns      INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (hour_utc, app, rule_id, route, server_id)
);
CREATE INDEX IF NOT EXISTS idx_rollup_hour ON rollup(hour_utc);
CREATE TABLE IF NOT EXISTS sessions (
	start_utc INTEGER NOT NULL,
	end_utc   INTEGER NOT NULL
);
`

// Open opens the stats database at path with WAL journal + NORMAL sync,
// applies the schema (idempotent), and seeds the schema_version row if absent.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	for _, p := range []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA foreign_keys=ON;",
	} {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&count); err != nil {
		_ = db.Close()
		return nil, err
	}
	if count == 0 {
		if _, err := db.Exec("INSERT INTO schema_version(v) VALUES(?)", schemaVersion); err != nil {
			_ = db.Close()
			return nil, err
		}
	} else if _, err := db.Exec("UPDATE schema_version SET v = ?", schemaVersion); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
