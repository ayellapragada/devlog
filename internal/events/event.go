package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Event represents a single development event
type Event struct {
	Version   int                    `json:"v"`
	ID        string                 `json:"id"`
	Timestamp string                 `json:"timestamp"`
	Source    string                 `json:"source"`
	Type      string                 `json:"type"`
	Repo      string                 `json:"repo,omitempty"`
	Branch    string                 `json:"branch,omitempty"`
	Payload   map[string]interface{} `json:"payload"`
}

// Valid source types
const (
	SourceGit       = "git"
	SourceShell     = "shell"
	SourceWisprflow = "wisprflow"
	SourceManual    = "manual"
	SourceGitHub    = "github"
)

// Valid event types
const (
	TypeCommit        = "commit"
	TypeMerge         = "merge"
	TypeCommand       = "command"
	TypeNote          = "note"
	TypePRMerged      = "pr_merged"
	TypeContextSwitch = "context_switch"
	TypeOther         = "other"
)

// NewEvent creates a new event with generated ID and current timestamp
func NewEvent(source, eventType string) *Event {
	return &Event{
		Version:   1,
		ID:        uuid.New().String(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Source:    source,
		Type:      eventType,
		Payload:   make(map[string]interface{}),
	}
}

// Validate checks if the event is valid
func (e *Event) Validate() error {
	if e.Version != 1 {
		return fmt.Errorf("unsupported version: %d", e.Version)
	}

	if e.ID == "" {
		return fmt.Errorf("id is required")
	}

	if _, err := uuid.Parse(e.ID); err != nil {
		return fmt.Errorf("invalid id format: %w", err)
	}

	if e.Timestamp == "" {
		return fmt.Errorf("timestamp is required")
	}

	if _, err := time.Parse(time.RFC3339, e.Timestamp); err != nil {
		return fmt.Errorf("invalid timestamp format (expected RFC3339): %w", err)
	}

	if !isValidSource(e.Source) {
		return fmt.Errorf("invalid source: %s", e.Source)
	}

	if !isValidType(e.Type) {
		return fmt.Errorf("invalid type: %s", e.Type)
	}

	if e.Payload == nil {
		return fmt.Errorf("payload is required")
	}

	return nil
}

// isValidSource checks if a source is valid
func isValidSource(source string) bool {
	switch source {
	case SourceGit, SourceShell, SourceWisprflow, SourceManual, SourceGitHub:
		return true
	default:
		return false
	}
}

// isValidType checks if an event type is valid
func isValidType(eventType string) bool {
	switch eventType {
	case TypeCommit, TypeMerge, TypeCommand, TypeNote, TypePRMerged, TypeContextSwitch, TypeOther:
		return true
	default:
		return false
	}
}

// ToJSON serializes the event to JSON
func (e *Event) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON deserializes an event from JSON
func FromJSON(data []byte) (*Event, error) {
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("unmarshal event: %w", err)
	}
	return &event, nil
}

// PayloadJSON returns the payload as a JSON string for storage
func (e *Event) PayloadJSON() (string, error) {
	data, err := json.Marshal(e.Payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	return string(data), nil
}
