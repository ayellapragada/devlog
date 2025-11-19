package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"devlog/internal/errors"
	"devlog/internal/events"
)

var ErrDuplicateEvent = fmt.Errorf("event already exists")

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "UNIQUE constraint failed") ||
		strings.Contains(errMsg, "constraint failed: UNIQUE")
}

func (s *Storage) InsertEvent(event *events.Event) error {
	return s.InsertEventContext(context.Background(), event)
}

func (s *Storage) InsertEventContext(ctx context.Context, event *events.Event) error {
	if err := event.Validate(); err != nil {
		return errors.WrapStorage("validate event", err)
	}

	payloadJSON, err := event.PayloadJSON()
	if err != nil {
		return errors.WrapStorage("serialize payload", err)
	}

	query := `
		INSERT INTO events (id, timestamp, source, type, repo, branch, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	ctx, cancel := context.WithTimeout(ctx, DefaultQueryTimeout)
	defer cancel()

	timestamp, err := time.Parse(time.RFC3339, event.Timestamp)
	if err != nil {
		return errors.WrapStorage("parse timestamp", err)
	}

	_, err = s.db.ExecContext(
		ctx,
		query,
		event.ID,
		timestamp.Unix(),
		event.Source,
		event.Type,
		event.Repo,
		event.Branch,
		payloadJSON,
		time.Now().Unix(),
	)

	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrDuplicateEvent
		}
		return errors.WrapStorage("insert event", err)
	}

	return nil
}

func (s *Storage) GetEvent(id string) (*events.Event, error) {
	return s.GetEventContext(context.Background(), id)
}

func (s *Storage) GetEventContext(ctx context.Context, id string) (*events.Event, error) {
	query := `
		SELECT id, timestamp, source, type, repo, branch, payload
		FROM events
		WHERE id = ?
	`

	ctx, cancel := context.WithTimeout(ctx, DefaultQueryTimeout)
	defer cancel()

	event, err := s.scanEvent(s.db.QueryRowContext(ctx, query, id))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("event not found: %s", id)
	}
	if err != nil {
		return nil, errors.WrapStorage("query event", err)
	}

	return event, nil
}

type QueryOptions struct {
	StartTime *time.Time
	EndTime   *time.Time
	Source    string
	Limit     int
}

func (s *Storage) QueryEvents(opts QueryOptions) ([]*events.Event, error) {
	return s.QueryEventsContext(context.Background(), opts)
}

func (s *Storage) QueryEventsContext(ctx context.Context, opts QueryOptions) ([]*events.Event, error) {
	query := `
		SELECT id, timestamp, source, type, repo, branch, payload
		FROM events
		WHERE 1=1
	`

	args := []interface{}{}

	if opts.StartTime != nil {
		query += " AND timestamp >= ?"
		args = append(args, opts.StartTime.Unix())
	}

	if opts.EndTime != nil {
		query += " AND timestamp < ?"
		args = append(args, opts.EndTime.Unix())
	}

	if opts.Source != "" {
		query += " AND source = ?"
		args = append(args, opts.Source)
	}

	query += " ORDER BY timestamp DESC"

	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	}

	ctx, cancel := context.WithTimeout(ctx, DefaultQueryTimeoutLong)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, args...)
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
	return s.CountContext(context.Background())
}

func (s *Storage) CountContext(ctx context.Context) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultQueryTimeout)
	defer cancel()

	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&count)
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
	var timestampUnix int64

	err := scanner.Scan(
		&event.ID,
		&timestampUnix,
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
	event.Timestamp = time.Unix(timestampUnix, 0).UTC().Format(time.RFC3339)

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

type SourceCount struct {
	Source string
	Count  int
}

func (s *Storage) CountBySource(ctx context.Context) ([]SourceCount, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultQueryTimeout)
	defer cancel()

	query := `
		SELECT source, COUNT(*) as count
		FROM events
		GROUP BY source
		ORDER BY count DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query sources: %w", err)
	}
	defer rows.Close()

	var results []SourceCount
	for rows.Next() {
		var sc SourceCount
		if err := rows.Scan(&sc.Source, &sc.Count); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		results = append(results, sc)
	}

	return results, rows.Err()
}

type TimelinePoint struct {
	Hour  string
	Count int
}

func (s *Storage) TimelineLast7Days(ctx context.Context) ([]TimelinePoint, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultQueryTimeout)
	defer cancel()

	query := `
		SELECT
			strftime('%Y-%m-%d %H:00:00', datetime(timestamp, 'unixepoch')) as hour,
			COUNT(*) as count
		FROM events
		WHERE timestamp >= unixepoch('now', '-7 days')
		GROUP BY hour
		ORDER BY hour DESC
		LIMIT 168
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query timeline: %w", err)
	}
	defer rows.Close()

	var results []TimelinePoint
	for rows.Next() {
		var tp TimelinePoint
		if err := rows.Scan(&tp.Hour, &tp.Count); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		results = append(results, tp)
	}

	return results, rows.Err()
}

type RepoStats struct {
	Repo  string
	Count int
}

func (s *Storage) TopRepos(ctx context.Context, limit int) ([]RepoStats, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultQueryTimeout)
	defer cancel()

	query := `
		SELECT repo, COUNT(*) as count
		FROM events
		WHERE repo IS NOT NULL AND repo != ''
		GROUP BY repo
		ORDER BY count DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query repos: %w", err)
	}
	defer rows.Close()

	var results []RepoStats
	for rows.Next() {
		var rs RepoStats
		if err := rows.Scan(&rs.Repo, &rs.Count); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		results = append(results, rs)
	}

	return results, rows.Err()
}

type CommandStats struct {
	Command string
	Count   int
}

func (s *Storage) TopCommands(ctx context.Context, limit int) ([]CommandStats, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultQueryTimeout)
	defer cancel()

	query := `
		SELECT
			json_extract(payload, '$.command') as command,
			COUNT(*) as count
		FROM events
		WHERE source = 'shell' AND type = 'command'
		AND json_extract(payload, '$.command') IS NOT NULL
		GROUP BY command
		ORDER BY count DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query commands: %w", err)
	}
	defer rows.Close()

	var results []CommandStats
	for rows.Next() {
		var cs CommandStats
		if err := rows.Scan(&cs.Command, &cs.Count); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		results = append(results, cs)
	}

	return results, rows.Err()
}
