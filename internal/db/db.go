package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS folders (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  tab             TEXT NOT NULL,
  category        TEXT NOT NULL,
  category_path   TEXT NOT NULL,
  folder_name     TEXT NOT NULL,
  folder_path     TEXT NOT NULL UNIQUE,
  mtime           REAL NOT NULL,
  content_hash    TEXT NOT NULL,
  indexed_at      INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_folders_tab_cat ON folders(tab, category);

CREATE TABLE IF NOT EXISTS folder_details (
  folder_id     INTEGER PRIMARY KEY REFERENCES folders(id) ON DELETE CASCADE,
  title         TEXT NOT NULL,
  favorite      INTEGER NOT NULL DEFAULT 0,
  source_link   TEXT,
  content_json  TEXT NOT NULL DEFAULT '[]',
  thumb_path    TEXT,
  preview_paths TEXT NOT NULL DEFAULT '[]',
  tags_json     TEXT NOT NULL DEFAULT '[]'
);
`

func Open(path string) (*sql.DB, error) {
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := d.Ping(); err != nil {
		d.Close()
		return nil, err
	}
	if _, err := d.Exec(schema); err != nil {
		d.Close()
		return nil, err
	}
	if err := ensureColumn(d, "folder_details", "tags_json", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
		d.Close()
		return nil, err
	}
	if err := ensureColumn(d, "folder_details", "description", "TEXT NOT NULL DEFAULT ''"); err != nil {
		d.Close()
		return nil, err
	}
	if err := ensureColumn(d, "folder_details", "hidden", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

// ensureColumn adds a column if it doesn't already exist (SQLite has no IF NOT EXISTS for ALTER).
func ensureColumn(d *sql.DB, table, column, definition string) error {
	rows, err := d.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = d.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	return err
}
