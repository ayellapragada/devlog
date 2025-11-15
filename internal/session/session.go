package session

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SessionStatus represents the current status of a session
type SessionStatus string

const (
	StatusActive    SessionStatus = "active"
	StatusCompleted SessionStatus = "completed"
	StatusArchived  SessionStatus = "archived"
)

// SessionTrigger represents how a session was initiated or ended
type SessionTrigger string

const (
	TriggerManual  SessionTrigger = "manual"
	TriggerPRMerge SessionTrigger = "pr_merge"
	TriggerRebase  SessionTrigger = "rebase"
	TriggerAuto    SessionTrigger = "auto"
	TriggerIdle    SessionTrigger = "idle_timeout"
)

// Session represents a cohesive work session containing related events
type Session struct {
	ID           string                 `json:"id"`
	StartTime    time.Time              `json:"start_time"`
	EndTime      *time.Time             `json:"end_time,omitempty"`
	Status       SessionStatus          `json:"status"`
	StartTrigger SessionTrigger         `json:"start_trigger"`
	EndTrigger   SessionTrigger         `json:"end_trigger,omitempty"`
	Description  string                 `json:"description,omitempty"`
	PrimaryRepo  string                 `json:"primary_repo,omitempty"`
	Branch       string                 `json:"branch,omitempty"`
	EventIDs     []string               `json:"event_ids"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// NewSession creates a new session with the given trigger
func NewSession(trigger SessionTrigger) *Session {
	return &Session{
		ID:           uuid.New().String(),
		StartTime:    time.Now().UTC(),
		Status:       StatusActive,
		StartTrigger: trigger,
		EventIDs:     []string{},
		Metadata:     make(map[string]interface{}),
	}
}

// NewManualSession creates a new manually-triggered session with optional description
func NewManualSession(description string) *Session {
	session := NewSession(TriggerManual)
	session.Description = description
	return session
}

// AddEvent adds an event ID to this session
func (s *Session) AddEvent(eventID string) {
	s.EventIDs = append(s.EventIDs, eventID)
}

// Complete marks the session as completed with the given trigger
func (s *Session) Complete(trigger SessionTrigger) error {
	if s.Status != StatusActive {
		return fmt.Errorf("cannot complete session: status is %s, expected %s", s.Status, StatusActive)
	}

	now := time.Now().UTC()
	s.EndTime = &now
	s.Status = StatusCompleted
	s.EndTrigger = trigger
	return nil
}

// Archive marks the session as archived
func (s *Session) Archive() error {
	if s.Status == StatusArchived {
		return fmt.Errorf("session is already archived")
	}

	s.Status = StatusArchived
	return nil
}

// Duration returns the duration of the session
// If the session is still active, returns duration up to now
func (s *Session) Duration() time.Duration {
	if s.EndTime == nil {
		return time.Since(s.StartTime)
	}
	return s.EndTime.Sub(s.StartTime)
}

// IsActive returns true if the session is currently active
func (s *Session) IsActive() bool {
	return s.Status == StatusActive
}

// Validate checks if the session is valid
func (s *Session) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("id is required")
	}

	if _, err := uuid.Parse(s.ID); err != nil {
		return fmt.Errorf("invalid id format: %w", err)
	}

	if s.StartTime.IsZero() {
		return fmt.Errorf("start_time is required")
	}

	if !isValidStatus(s.Status) {
		return fmt.Errorf("invalid status: %s", s.Status)
	}

	if !isValidTrigger(s.StartTrigger) {
		return fmt.Errorf("invalid start_trigger: %s", s.StartTrigger)
	}

	if s.EndTrigger != "" && !isValidTrigger(s.EndTrigger) {
		return fmt.Errorf("invalid end_trigger: %s", s.EndTrigger)
	}

	if s.Status != StatusActive && s.EndTime == nil {
		return fmt.Errorf("completed/archived session must have end_time")
	}

	if s.Status == StatusActive && s.EndTime != nil {
		return fmt.Errorf("active session cannot have end_time")
	}

	return nil
}

// isValidStatus checks if a status is valid
func isValidStatus(status SessionStatus) bool {
	switch status {
	case StatusActive, StatusCompleted, StatusArchived:
		return true
	default:
		return false
	}
}

// isValidTrigger checks if a trigger is valid
func isValidTrigger(trigger SessionTrigger) bool {
	switch trigger {
	case TriggerManual, TriggerPRMerge, TriggerRebase, TriggerAuto, TriggerIdle:
		return true
	default:
		return false
	}
}

// ToJSON serializes the session to JSON
func (s *Session) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

// FromJSON deserializes a session from JSON
func FromJSON(data []byte) (*Session, error) {
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &session, nil
}

// MetadataJSON returns the metadata as a JSON string for storage
func (s *Session) MetadataJSON() (string, error) {
	if s.Metadata == nil {
		return "{}", nil
	}
	data, err := json.Marshal(s.Metadata)
	if err != nil {
		return "", fmt.Errorf("marshal metadata: %w", err)
	}
	return string(data), nil
}
