package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type EventSource string

const (
	SourceGit       EventSource = "git"
	SourceShell     EventSource = "shell"
	SourceWisprflow EventSource = "wisprflow"
	SourceManual    EventSource = "manual"
	SourceGitHub    EventSource = "github"
	SourceClipboard EventSource = "clipboard"
	SourceTmux      EventSource = "tmux"
	SourceClaude    EventSource = "claude"
)

func (s EventSource) String() string {
	return string(s)
}

func (s EventSource) Validate() error {
	switch s {
	case SourceGit, SourceShell, SourceWisprflow, SourceManual, SourceGitHub, SourceClipboard, SourceTmux, SourceClaude:
		return nil
	default:
		return fmt.Errorf("invalid source: %s", s)
	}
}

type EventType string

const (
	TypeCommit        EventType = "commit"
	TypeMerge         EventType = "merge"
	TypePush          EventType = "push"
	TypePull          EventType = "pull"
	TypeFetch         EventType = "fetch"
	TypeCheckout      EventType = "checkout"
	TypeRebase        EventType = "rebase"
	TypeStash         EventType = "stash"
	TypeCommand       EventType = "command"
	TypeNote          EventType = "note"
	TypePRMerged      EventType = "pr_merged"
	TypeContextSwitch EventType = "context_switch"
	TypeTranscription EventType = "transcription"
	TypeCopy          EventType = "copy"
	TypeTmuxSession   EventType = "tmux_session"
	TypeTmuxWindow    EventType = "tmux_window"
	TypeTmuxPane      EventType = "tmux_pane"
	TypeTmuxAttach    EventType = "tmux_attach"
	TypeTmuxDetach    EventType = "tmux_detach"
	TypeConversation  EventType = "conversation"
	TypeFileEdit      EventType = "file_edit"
	TypeOther         EventType = "other"
)

func (t EventType) String() string {
	return string(t)
}

func (t EventType) Validate() error {
	switch t {
	case TypeCommit, TypeMerge, TypePush, TypePull, TypeFetch, TypeCheckout, TypeRebase, TypeStash,
		TypeCommand, TypeNote, TypePRMerged, TypeContextSwitch, TypeTranscription, TypeCopy,
		TypeTmuxSession, TypeTmuxWindow, TypeTmuxPane, TypeTmuxAttach, TypeTmuxDetach,
		TypeConversation, TypeFileEdit,
		TypeOther:
		return nil
	default:
		return fmt.Errorf("invalid type: %s", t)
	}
}

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

	if err := EventSource(e.Source).Validate(); err != nil {
		return err
	}

	if err := EventType(e.Type).Validate(); err != nil {
		return err
	}

	if e.Payload == nil {
		return fmt.Errorf("payload is required")
	}

	return nil
}

func (e *Event) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

func FromJSON(data []byte) (*Event, error) {
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("unmarshal event: %w", err)
	}
	return &event, nil
}

func (e *Event) PayloadJSON() (string, error) {
	data, err := json.Marshal(e.Payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	return string(data), nil
}
