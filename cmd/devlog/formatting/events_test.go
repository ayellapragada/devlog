package formatting

import (
	"testing"

	"devlog/internal/events"
)

func TestGetTypeTag(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		expected  string
	}{
		{"commit", "commit", "git"},
		{"merge", "merge", "git"},
		{"command", "command", "shell"},
		{"note", "note", "note"},
		{"pr_merged", "pr_merged", "github"},
		{"transcription", "transcription", "voice"},
		{"unknown", "unknown_type", "unknown_type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := events.NewEvent("test", tt.eventType)
			result := getTypeTag(event)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetFolder(t *testing.T) {
	tests := []struct {
		name     string
		repo     string
		workdir  string
		expected string
	}{
		{
			name:     "with repo",
			repo:     "/Users/test/projects/myapp",
			expected: "myapp",
		},
		{
			name:     "no repo or workdir",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := events.NewEvent("test", "commit")
			event.Repo = tt.repo
			if tt.workdir != "" {
				event.Payload["workdir"] = tt.workdir
			}

			result := getFolder(event)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetFolderWithWorkdir(t *testing.T) {
	event := events.NewEvent("shell", "command")
	event.Payload["workdir"] = "/Users/test/projects/myapp"

	result := getFolder(event)
	if result != "myapp" {
		t.Errorf("Expected myapp, got %s", result)
	}
}

func TestGetFolderRepoTakesPrecedence(t *testing.T) {
	event := events.NewEvent("shell", "command")
	event.Repo = "/Users/test/projects/repo1"
	event.Payload["workdir"] = "/Users/test/projects/repo2"

	result := getFolder(event)
	if result != "repo1" {
		t.Errorf("Expected repo1 (repo should take precedence), got %s", result)
	}
}
