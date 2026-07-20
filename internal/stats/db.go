// Package stats persists connection records and hourly rollups to SQLite.
package stats

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const schemaVersion = 1

const schema = `
CREATE TABLE IF NOT EXISTS schema_version (v INTEGER NOT NULL);

CREATE TABLE IF NOT EXISTS connections (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	started_at      INTEGER NOT NULL,
	ended_at        INTEGER,
	process_name    TEXT,
	src_addr        TEXT,
	dst_domain      TEXT,
	dst_ip          TEXT,
	dst_port        INTEGER,
	protocol        TEXT,
	rule_id         TEXT,
	action          TEXT,
	server_id       TEXT,
	bytes_up        INTEGER DEFAULT 0,
	bytes_down      INTEGER DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_conn_started   ON connections(started_at);
CREATE INDEX IF NOT EXISTS idx_conn_ended     ON connections(ended_at);
CREATE INDEX IF NOT EXISTS idx_conn_process   ON connections(process_name);
CREATE INDEX IF NOT EXISTS idx_conn_domain    ON connections(dst_domain);

CREATE TABLE IF NOT EXISTS hourly_rollup (
	hour_utc        INTEGER NOT NULL,
	dim_kind        TEXT    NOT NULL,
	dim_value       TEXT    NOT NULL,
	bytes_up        INTEGER NOT NULL,
	bytes_down      INTEGER NOT NULL,
	connections     INTEGER NOT NULL,
	PRIMARY KEY (hour_utc, dim_kind, dim_value)
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
	}
	return db, nil
}
