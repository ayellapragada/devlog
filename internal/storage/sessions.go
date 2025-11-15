package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"devlog/internal/session"
)

func (s *Storage) InsertSession(sess *session.Session) error {
	if err := sess.Validate(); err != nil {
		return fmt.Errorf("invalid session: %w", err)
	}

	metadataJSON, err := sess.MetadataJSON()
	if err != nil {
		return fmt.Errorf("serialize metadata: %w", err)
	}

	query := `
		INSERT INTO sessions (
			id, start_time, end_time, status, start_trigger, end_trigger,
			description, primary_repo, branch, metadata, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var endTime *int64
	if sess.EndTime != nil {
		ts := sess.EndTime.Unix()
		endTime = &ts
	}

	var endTrigger *string
	if sess.EndTrigger != "" {
		trigger := string(sess.EndTrigger)
		endTrigger = &trigger
	}

	_, err = s.db.Exec(
		query,
		sess.ID,
		sess.StartTime.Unix(),
		endTime,
		string(sess.Status),
		string(sess.StartTrigger),
		endTrigger,
		sess.Description,
		sess.PrimaryRepo,
		sess.Branch,
		metadataJSON,
		time.Now().Unix(),
	)

	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}

	return nil
}

func (s *Storage) UpdateSession(sess *session.Session) error {
	if err := sess.Validate(); err != nil {
		return fmt.Errorf("invalid session: %w", err)
	}

	metadataJSON, err := sess.MetadataJSON()
	if err != nil {
		return fmt.Errorf("serialize metadata: %w", err)
	}

	query := `
		UPDATE sessions
		SET end_time = ?, status = ?, end_trigger = ?,
		    description = ?, primary_repo = ?, branch = ?, metadata = ?
		WHERE id = ?
	`

	var endTime *int64
	if sess.EndTime != nil {
		ts := sess.EndTime.Unix()
		endTime = &ts
	}

	var endTrigger *string
	if sess.EndTrigger != "" {
		trigger := string(sess.EndTrigger)
		endTrigger = &trigger
	}

	result, err := s.db.Exec(
		query,
		endTime,
		string(sess.Status),
		endTrigger,
		sess.Description,
		sess.PrimaryRepo,
		sess.Branch,
		metadataJSON,
		sess.ID,
	)

	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("session not found: %s", sess.ID)
	}

	return nil
}

func (s *Storage) GetSession(id string) (*session.Session, error) {
	query := `
		SELECT id, start_time, end_time, status, start_trigger, end_trigger,
		       description, primary_repo, branch, metadata
		FROM sessions
		WHERE id = ?
	`

	sess, err := s.scanSession(s.db.QueryRow(query, id))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}

	eventIDs, err := s.GetSessionEvents(id)
	if err != nil {
		return nil, fmt.Errorf("load session events: %w", err)
	}
	sess.EventIDs = eventIDs

	return sess, nil
}

func (s *Storage) GetActiveSession() (*session.Session, error) {
	query := `
		SELECT id, start_time, end_time, status, start_trigger, end_trigger,
		       description, primary_repo, branch, metadata
		FROM sessions
		WHERE status = ?
		ORDER BY start_time DESC
		LIMIT 1
	`

	sess, err := s.scanSession(s.db.QueryRow(query, string(session.StatusActive)))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query active session: %w", err)
	}

	eventIDs, err := s.GetSessionEvents(sess.ID)
	if err != nil {
		return nil, fmt.Errorf("load session events: %w", err)
	}
	sess.EventIDs = eventIDs

	return sess, nil
}

func (s *Storage) ListSessions(limit int, status session.SessionStatus) ([]*session.Session, error) {
	query := `
		SELECT id, start_time, end_time, status, start_trigger, end_trigger,
		       description, primary_repo, branch, metadata
		FROM sessions
		WHERE 1=1
	`

	args := []interface{}{}

	if status != "" {
		query += " AND status = ?"
		args = append(args, string(status))
	}

	query += " ORDER BY start_time DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var result []*session.Session

	for rows.Next() {
		sess, err := s.scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}

		eventIDs, err := s.GetSessionEvents(sess.ID)
		if err != nil {
			return nil, fmt.Errorf("load session events: %w", err)
		}
		sess.EventIDs = eventIDs

		result = append(result, sess)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}

	return result, nil
}

func (s *Storage) GetSessionEvents(sessionID string) ([]string, error) {
	query := `
		SELECT event_id
		FROM session_events
		WHERE session_id = ?
		ORDER BY created_at ASC
	`

	rows, err := s.db.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query session events: %w", err)
	}
	defer rows.Close()

	var eventIDs []string
	for rows.Next() {
		var eventID string
		if err := rows.Scan(&eventID); err != nil {
			return nil, fmt.Errorf("scan event id: %w", err)
		}
		eventIDs = append(eventIDs, eventID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate event ids: %w", err)
	}

	return eventIDs, nil
}

func (s *Storage) AddEventToSession(sessionID, eventID string) error {
	query := `
		INSERT INTO session_events (session_id, event_id, created_at)
		VALUES (?, ?, ?)
	`

	_, err := s.db.Exec(query, sessionID, eventID, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("insert session event: %w", err)
	}

	return nil
}

func (s *Storage) scanSession(scanner interface {
	Scan(dest ...interface{}) error
}) (*session.Session, error) {
	var sess session.Session
	var endTime sql.NullInt64
	var endTrigger, description, primaryRepo, branch sql.NullString
	var metadataJSON string

	var startTime int64
	err := scanner.Scan(
		&sess.ID,
		&startTime,
		&endTime,
		&sess.Status,
		&sess.StartTrigger,
		&endTrigger,
		&description,
		&primaryRepo,
		&branch,
		&metadataJSON,
	)

	if err != nil {
		return nil, err
	}

	sess.StartTime = time.Unix(startTime, 0).UTC()

	if endTime.Valid {
		t := time.Unix(endTime.Int64, 0).UTC()
		sess.EndTime = &t
	}

	if endTrigger.Valid {
		sess.EndTrigger = session.SessionTrigger(endTrigger.String)
	}

	if description.Valid {
		sess.Description = description.String
	}

	if primaryRepo.Valid {
		sess.PrimaryRepo = primaryRepo.String
	}

	if branch.Valid {
		sess.Branch = branch.String
	}

	// Parse metadata JSON
	if metadataJSON != "" && metadataJSON != "{}" {
		if err := json.Unmarshal([]byte(metadataJSON), &sess.Metadata); err != nil {
			return nil, fmt.Errorf("parse metadata: %w", err)
		}
	} else {
		sess.Metadata = make(map[string]interface{})
	}

	// EventIDs will be loaded separately
	sess.EventIDs = []string{}

	return &sess, nil
}
