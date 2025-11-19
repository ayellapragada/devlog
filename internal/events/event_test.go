package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewEvent(t *testing.T) {
	event := NewEvent(string(SourceGit), string(TypeCommit))

	if event.Version != 1 {
		t.Errorf("got version %d, want 1", event.Version)
	}

	if event.Source != string(SourceGit) {
		t.Errorf("got source %s, want %s", event.Source, SourceGit)
	}

	if event.Type != string(TypeCommit) {
		t.Errorf("got type %s, want %s", event.Type, TypeCommit)
	}

	if _, err := uuid.Parse(event.ID); err != nil {
		t.Errorf("invalid UUID: %v", err)
	}

	if _, err := time.Parse(time.RFC3339, event.Timestamp); err != nil {
		t.Errorf("invalid timestamp: %v", err)
	}

	if event.Payload == nil {
		t.Error("payload should not be nil")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		event   *Event
		wantErr bool
	}{
		{
			name: "valid event",
			event: &Event{
				Version:   1,
				ID:        uuid.New().String(),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Source:    string(SourceGit),
				Type:      string(TypeCommit),
				Payload:   map[string]interface{}{"hash": "abc123"},
			},
			wantErr: false,
		},
		{
			name: "invalid version",
			event: &Event{
				Version:   2,
				ID:        uuid.New().String(),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Source:    string(SourceGit),
				Type:      string(TypeCommit),
				Payload:   map[string]interface{}{},
			},
			wantErr: true,
		},
		{
			name: "missing id",
			event: &Event{
				Version:   1,
				ID:        "",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Source:    string(SourceGit),
				Type:      string(TypeCommit),
				Payload:   map[string]interface{}{},
			},
			wantErr: true,
		},
		{
			name: "invalid id format",
			event: &Event{
				Version:   1,
				ID:        "not-a-uuid",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Source:    string(SourceGit),
				Type:      string(TypeCommit),
				Payload:   map[string]interface{}{},
			},
			wantErr: true,
		},
		{
			name: "missing timestamp",
			event: &Event{
				Version: 1,
				ID:      uuid.New().String(),
				Source:  string(SourceGit),
				Type:    string(TypeCommit),
				Payload: map[string]interface{}{},
			},
			wantErr: true,
		},
		{
			name: "invalid timestamp format",
			event: &Event{
				Version:   1,
				ID:        uuid.New().String(),
				Timestamp: "not-a-timestamp",
				Source:    string(SourceGit),
				Type:      string(TypeCommit),
				Payload:   map[string]interface{}{},
			},
			wantErr: true,
		},
		{
			name: "invalid source",
			event: &Event{
				Version:   1,
				ID:        uuid.New().String(),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Source:    "invalid",
				Type:      string(TypeCommit),
				Payload:   map[string]interface{}{},
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			event: &Event{
				Version:   1,
				ID:        uuid.New().String(),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Source:    string(SourceGit),
				Type:      "invalid",
				Payload:   map[string]interface{}{},
			},
			wantErr: true,
		},
		{
			name: "nil payload",
			event: &Event{
				Version:   1,
				ID:        uuid.New().String(),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Source:    string(SourceGit),
				Type:      string(TypeCommit),
				Payload:   nil,
			},
			wantErr: true,
		},
		{
			name: "with optional fields",
			event: &Event{
				Version:   1,
				ID:        uuid.New().String(),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Source:    string(SourceGit),
				Type:      string(TypeCommit),
				Repo:      "/path/to/repo",
				Branch:    "main",
				Payload:   map[string]interface{}{"hash": "abc123"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToJSONFromJSON(t *testing.T) {
	original := &Event{
		Version:   1,
		ID:        uuid.New().String(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Source:    string(SourceGit),
		Type:      string(TypeCommit),
		Repo:      "/path/to/repo",
		Branch:    "main",
		Payload: map[string]interface{}{
			"hash":    "abc123",
			"message": "Test commit",
			"author":  "Test Author",
		},
	}

	// Serialize
	data, err := original.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	// Deserialize
	restored, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON() error: %v", err)
	}

	// Compare
	if restored.Version != original.Version {
		t.Errorf("got version %d, want %d", restored.Version, original.Version)
	}

	if restored.ID != original.ID {
		t.Errorf("got ID %s, want %s", restored.ID, original.ID)
	}

	if restored.Source != original.Source {
		t.Errorf("got source %s, want %s", restored.Source, original.Source)
	}

	if restored.Type != original.Type {
		t.Errorf("got type %s, want %s", restored.Type, original.Type)
	}

	if restored.Repo != original.Repo {
		t.Errorf("got repo %s, want %s", restored.Repo, original.Repo)
	}

	if restored.Branch != original.Branch {
		t.Errorf("got branch %s, want %s", restored.Branch, original.Branch)
	}

	// Check payload
	if restored.Payload["hash"] != original.Payload["hash"] {
		t.Errorf("got hash %v, want %v", restored.Payload["hash"], original.Payload["hash"])
	}
}

func TestPayloadJSON(t *testing.T) {
	event := NewEvent(string(SourceGit), string(TypeCommit))
	event.Payload["hash"] = "abc123"
	event.Payload["message"] = "Test commit"

	payloadJSON, err := event.PayloadJSON()
	if err != nil {
		t.Fatalf("PayloadJSON() error: %v", err)
	}

	// Parse it back
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if payload["hash"] != "abc123" {
		t.Errorf("got hash %v, want abc123", payload["hash"])
	}

	if payload["message"] != "Test commit" {
		t.Errorf("got message %v, want 'Test commit'", payload["message"])
	}
}
