package store

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func InitDB(path string) (*sql.DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db dir %s: %w", dir, err)
	}
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", path))
	if err != nil {
		return nil, fmt.Errorf("open db %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func runMigrations(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	if err := backfillMigrations(db); err != nil {
		return fmt.Errorf("backfill migrations: %w", err)
	}

	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}

		var alreadyApplied int
		if err := db.QueryRow("SELECT 1 FROM schema_migrations WHERE name = ?", e.Name()).Scan(&alreadyApplied); err == nil {
			log.Printf("[store] migration already applied: %s", e.Name())
			continue
		} else if err != sql.ErrNoRows {
			return fmt.Errorf("check migration %s: %w", e.Name(), err)
		}

		content, err := migrationFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return err
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", e.Name(), err)
		}
		if _, err := tx.Exec(string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %s: %w", e.Name(), err)
		}
		if _, err := tx.Exec("INSERT INTO schema_migrations (name) VALUES (?)", e.Name()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", e.Name(), err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", e.Name(), err)
		}
		log.Printf("[store] applied migration: %s", e.Name())
	}
	return nil
}

// backfillMigrations marks existing migrations as applied for databases that
// predate the schema_migrations table. It inspects the current schema so old
// databases migrate cleanly without re-running ALTER statements.
func backfillMigrations(db *sql.DB) error {
	type check struct {
		name string
		done func(*sql.DB) bool
	}
	checks := []check{
		{
			name: "001_init.sql",
			done: func(db *sql.DB) bool { return tableExists(db, "sessions") },
		},
		{
			name: "002_authorized_devices.sql",
			done: func(db *sql.DB) bool { return tableExists(db, "authorized_devices") },
		},
		{
			name: "003_device_alias.sql",
			done: func(db *sql.DB) bool { return columnExists(db, "authorized_devices", "alias") },
		},
		{
			name: "004_agent_version.sql",
			done: func(db *sql.DB) bool { return columnExists(db, "agents", "version") },
		},
	}
	for _, c := range checks {
		var alreadyApplied int
		if err := db.QueryRow("SELECT 1 FROM schema_migrations WHERE name = ?", c.name).Scan(&alreadyApplied); err == nil {
			continue
		} else if err != sql.ErrNoRows {
			return fmt.Errorf("check migration %s: %w", c.name, err)
		}
		if !c.done(db) {
			continue
		}
		if _, err := db.Exec("INSERT INTO schema_migrations (name) VALUES (?)", c.name); err != nil {
			return fmt.Errorf("record migration %s: %w", c.name, err)
		}
		log.Printf("[store] backfilled migration: %s", c.name)
	}
	return nil
}

func tableExists(db *sql.DB, name string) bool {
	var count int
	if err := db.QueryRow(
		"SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name,
	).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

func columnExists(db *sql.DB, table, column string) bool {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			continue
		}
		if name == column {
			return true
		}
	}
	return false
}
