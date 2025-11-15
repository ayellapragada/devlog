package session

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type SessionStatus string

const (
	StatusActive    SessionStatus = "active"
	StatusCompleted SessionStatus = "completed"
	StatusArchived  SessionStatus = "archived"
)

type SessionTrigger string

const (
	TriggerManual  SessionTrigger = "manual"
	TriggerPRMerge SessionTrigger = "pr_merge"
	TriggerRebase  SessionTrigger = "rebase"
	TriggerAuto    SessionTrigger = "auto"
	TriggerIdle    SessionTrigger = "idle_timeout"
)

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

func NewManualSession(description string) *Session {
	session := NewSession(TriggerManual)
	session.Description = description
	return session
}

func (s *Session) AddEvent(eventID string) {
	s.EventIDs = append(s.EventIDs, eventID)
}

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

func (s *Session) Archive() error {
	if s.Status == StatusArchived {
		return fmt.Errorf("session is already archived")
	}

	s.Status = StatusArchived
	return nil
}

func (s *Session) Duration() time.Duration {
	if s.EndTime == nil {
		return time.Since(s.StartTime)
	}
	return s.EndTime.Sub(s.StartTime)
}

func (s *Session) IsActive() bool {
	return s.Status == StatusActive
}

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

func isValidStatus(status SessionStatus) bool {
	switch status {
	case StatusActive, StatusCompleted, StatusArchived:
		return true
	default:
		return false
	}
}

func isValidTrigger(trigger SessionTrigger) bool {
	switch trigger {
	case TriggerManual, TriggerPRMerge, TriggerRebase, TriggerAuto, TriggerIdle:
		return true
	default:
		return false
	}
}

func (s *Session) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

func FromJSON(data []byte) (*Session, error) {
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &session, nil
}

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
