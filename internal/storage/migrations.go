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
		Description: "Add sessions and session_events tables",
		Up: `
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			start_time INTEGER NOT NULL,
			end_time INTEGER,
			status TEXT NOT NULL,
			start_trigger TEXT NOT NULL,
			end_trigger TEXT,
			description TEXT,
			primary_repo TEXT,
			branch TEXT,
			metadata TEXT NOT NULL,
			created_at INTEGER NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_session_status ON sessions(status);
		CREATE INDEX IF NOT EXISTS idx_session_start_time ON sessions(start_time);
		CREATE INDEX IF NOT EXISTS idx_session_primary_repo ON sessions(primary_repo);

		CREATE TABLE IF NOT EXISTS session_events (
			session_id TEXT NOT NULL,
			event_id TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			PRIMARY KEY (session_id, event_id),
			FOREIGN KEY (session_id) REFERENCES sessions(id),
			FOREIGN KEY (event_id) REFERENCES events(id)
		);

		CREATE INDEX IF NOT EXISTS idx_session_events_session ON session_events(session_id);
		CREATE INDEX IF NOT EXISTS idx_session_events_event ON session_events(event_id);
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

func RunMigrations(db *sql.DB) error {
	currentVersion, err := getCurrentVersion(db)
	if err != nil {
		return fmt.Errorf("get current version: %w", err)
	}

	for _, migration := range migrations {
		if migration.Version <= currentVersion {
			continue
		}

		fmt.Printf("Applying migration %d: %s\n", migration.Version, migration.Description)

		if _, err := db.Exec(migration.Up); err != nil {
			return fmt.Errorf("apply migration %d: %w", migration.Version, err)
		}

		if err := setVersion(db, migration.Version); err != nil {
			return fmt.Errorf("record version %d: %w", migration.Version, err)
		}

		fmt.Printf("Migration %d applied successfully\n", migration.Version)
	}

	if currentVersion == len(migrations) {
		fmt.Println("Database is up to date")
	}

	return nil
}

func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}
