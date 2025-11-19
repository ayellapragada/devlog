package storage

import (
	"database/sql"
	"fmt"
	"time"
)

type Migration struct {
	Version     int
	Description string
	Up          string
}

var migrations = []Migration{
	{
		Version:     1,
		Description: "Initial schema with events table",
		Up: `
		CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			timestamp TEXT NOT NULL,
			source TEXT NOT NULL,
			type TEXT NOT NULL,
			repo TEXT,
			branch TEXT,
			payload TEXT NOT NULL,
			created_at INTEGER NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_timestamp ON events(timestamp);
		CREATE INDEX IF NOT EXISTS idx_repo ON events(repo);
		CREATE INDEX IF NOT EXISTS idx_source ON events(source);
		CREATE INDEX IF NOT EXISTS idx_created_at ON events(created_at);
		`,
	},
	{
		Version:     2,
		Description: "Add FTS5 full-text search and JSON1 support",
		Up: `
		CREATE TABLE events_new (
			id TEXT PRIMARY KEY,
			timestamp INTEGER NOT NULL,
			source TEXT NOT NULL,
			type TEXT NOT NULL,
			repo TEXT,
			branch TEXT,
			payload JSON NOT NULL,
			created_at INTEGER NOT NULL
		);

		INSERT INTO events_new
		SELECT
			id,
			CAST(strftime('%s', timestamp) AS INTEGER) as timestamp,
			source,
			type,
			repo,
			branch,
			payload,
			created_at
		FROM events;

		DROP TABLE events;
		ALTER TABLE events_new RENAME TO events;

		CREATE INDEX idx_events_timestamp ON events(timestamp);
		CREATE INDEX idx_events_source ON events(source);
		CREATE INDEX idx_events_type ON events(type);
		CREATE INDEX idx_events_repo_branch ON events(repo, branch);

		CREATE VIRTUAL TABLE events_fts USING fts5(
			id UNINDEXED,
			source,
			type,
			payload,
			content=events,
			content_rowid=rowid,
			tokenize='porter unicode61 remove_diacritics 2'
		);

		INSERT INTO events_fts(rowid, id, source, type, payload)
		SELECT rowid, id, source, type, payload FROM events;

		CREATE TRIGGER events_ai AFTER INSERT ON events BEGIN
			INSERT INTO events_fts(rowid, id, source, type, payload)
			VALUES (new.rowid, new.id, new.source, new.type, new.payload);
		END;

		CREATE TRIGGER events_ad AFTER DELETE ON events BEGIN
			DELETE FROM events_fts WHERE rowid = old.rowid;
		END;

		CREATE TRIGGER events_au AFTER UPDATE ON events BEGIN
			DELETE FROM events_fts WHERE rowid = old.rowid;
			INSERT INTO events_fts(rowid, id, source, type, payload)
			VALUES (new.rowid, new.id, new.source, new.type, new.payload);
		END;
		`,
	},
}

func getCurrentVersion(db *sql.DB) (int, error) {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY,
			applied_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return 0, fmt.Errorf("create schema_version table: %w", err)
	}

	var version int
	err = db.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	if err == sql.ErrNoRows || version == 0 {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("query version: %w", err)
	}

	return version, nil
}

func setVersion(db *sql.DB, version int) error {
	_, err := db.Exec("INSERT INTO schema_version (version, applied_at) VALUES (?, ?)",
		version, getCurrentTimestamp())
	return err
}

type MigrationLogger interface {
	Printf(format string, v ...interface{})
}

func RunMigrations(db *sql.DB, logger MigrationLogger) error {
	currentVersion, err := getCurrentVersion(db)
	if err != nil {
		return fmt.Errorf("get current version: %w", err)
	}

	migrationRan := false
	for _, migration := range migrations {
		if migration.Version <= currentVersion {
			continue
		}

		migrationRan = true
		if logger != nil {
			logger.Printf("Applying migration %d: %s", migration.Version, migration.Description)
		}

		if _, err := db.Exec(migration.Up); err != nil {
			return fmt.Errorf("apply migration %d: %w", migration.Version, err)
		}

		if err := setVersion(db, migration.Version); err != nil {
			return fmt.Errorf("record version %d: %w", migration.Version, err)
		}

		if logger != nil {
			logger.Printf("Migration %d applied successfully", migration.Version)
		}
	}

	if !migrationRan && logger != nil {
		logger.Printf("Database is up to date")
	}

	return nil
}

func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}
