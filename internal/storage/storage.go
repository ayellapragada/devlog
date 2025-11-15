package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"devlog/internal/events"
)

type Storage struct {
	db *sql.DB
}

func New(dbPath string) (*Storage, error) {
	_, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("database does not exist at %s (run with --init to create)", dbPath)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	if err := RunMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &Storage{db: db}, nil
}

func InitDB(dbPath string) error {
	if _, err := os.Stat(dbPath); err == nil {
		return fmt.Errorf("database already exists at %s", dbPath)
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("enable WAL mode: %w", err)
	}

	schema := `
	CREATE TABLE events (
		id TEXT PRIMARY KEY,
		timestamp TEXT NOT NULL,
		source TEXT NOT NULL,
		type TEXT NOT NULL,
		repo TEXT,
		branch TEXT,
		payload TEXT NOT NULL,
		created_at INTEGER NOT NULL
	);

	CREATE INDEX idx_timestamp ON events(timestamp);
	CREATE INDEX idx_repo ON events(repo);
	CREATE INDEX idx_source ON events(source);
	CREATE INDEX idx_created_at ON events(created_at);

	CREATE TABLE sessions (
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

	CREATE INDEX idx_session_status ON sessions(status);
	CREATE INDEX idx_session_start_time ON sessions(start_time);
	CREATE INDEX idx_session_primary_repo ON sessions(primary_repo);

	CREATE TABLE session_events (
		session_id TEXT NOT NULL,
		event_id TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		PRIMARY KEY (session_id, event_id),
		FOREIGN KEY (session_id) REFERENCES sessions(id),
		FOREIGN KEY (event_id) REFERENCES events(id)
	);

	CREATE INDEX idx_session_events_session ON session_events(session_id);
	CREATE INDEX idx_session_events_event ON session_events(event_id);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	fmt.Printf("Created database at %s\n", dbPath)
	return nil
}

func (s *Storage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Storage) InsertEvent(event *events.Event) error {
	if err := event.Validate(); err != nil {
		return fmt.Errorf("invalid event: %w", err)
	}

	payloadJSON, err := event.PayloadJSON()
	if err != nil {
		return fmt.Errorf("serialize payload: %w", err)
	}

	query := `
		INSERT INTO events (id, timestamp, source, type, repo, branch, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.Exec(
		query,
		event.ID,
		event.Timestamp,
		event.Source,
		event.Type,
		event.Repo,
		event.Branch,
		payloadJSON,
		time.Now().Unix(),
	)

	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	return nil
}

func (s *Storage) GetEvent(id string) (*events.Event, error) {
	query := `
		SELECT id, timestamp, source, type, repo, branch, payload
		FROM events
		WHERE id = ?
	`

	event, err := s.scanEvent(s.db.QueryRow(query, id))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("event not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query event: %w", err)
	}

	return event, nil
}

func (s *Storage) ListEvents(limit int, source string) ([]*events.Event, error) {
	query := `
		SELECT id, timestamp, source, type, repo, branch, payload
		FROM events
		WHERE 1=1
	`

	args := []interface{}{}

	if source != "" {
		query += " AND source = ?"
		args = append(args, source)
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var result []*events.Event

	for rows.Next() {
		event, err := s.scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		result = append(result, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}

	return result, nil
}

func (s *Storage) Count() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count events: %w", err)
	}
	return count, nil
}

func (s *Storage) scanEvent(scanner interface {
	Scan(dest ...interface{}) error
}) (*events.Event, error) {
	var event events.Event
	var payloadJSON string
	var repo, branch sql.NullString

	err := scanner.Scan(
		&event.ID,
		&event.Timestamp,
		&event.Source,
		&event.Type,
		&repo,
		&branch,
		&payloadJSON,
	)

	if err != nil {
		return nil, err
	}

	event.Version = 1

	if repo.Valid {
		event.Repo = repo.String
	}

	if branch.Valid {
		event.Branch = branch.String
	}

	restoredEvent, err := s.restoreEventPayload(&event, payloadJSON)
	if err != nil {
		return nil, fmt.Errorf("restore payload: %w", err)
	}

	return restoredEvent, nil
}

func (s *Storage) restoreEventPayload(event *events.Event, payloadJSON string) (*events.Event, error) {
	restoredEvent, err := events.FromJSON([]byte(fmt.Sprintf(`{"v":1,"id":"%s","timestamp":"%s","source":"%s","type":"%s","payload":%s}`,
		event.ID, event.Timestamp, event.Source, event.Type, payloadJSON)))
	if err != nil {
		return nil, err
	}

	restoredEvent.Repo = event.Repo
	restoredEvent.Branch = event.Branch

	return restoredEvent, nil
}
