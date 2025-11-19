package wisprflow

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetDBPath(t *testing.T) {
	homeDir := "/Users/testuser"

	tests := []struct {
		name       string
		configPath string
		want       string
	}{
		{
			name:       "default path with tilde",
			configPath: "~/Library/Application Support/Wispr Flow/flow.sqlite",
			want:       "/Users/testuser/Library/Application Support/Wispr Flow/flow.sqlite",
		},
		{
			name:       "empty config uses default",
			configPath: "",
			want:       "/Users/testuser/Library/Application Support/Wispr Flow/flow.sqlite",
		},
		{
			name:       "absolute path stays unchanged",
			configPath: "/absolute/path/to/flow.sqlite",
			want:       "/absolute/path/to/flow.sqlite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetDBPath(homeDir, tt.configPath)
			if got != tt.want {
				t.Errorf("GetDBPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPollDatabase_RealDatabase(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	dbPath := filepath.Join(homeDir, "Library", "Application Support", "Wispr Flow", "flow.sqlite")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Skip("Wispr Flow database not found - skipping test")
	}

	// Test polling from now (should return no entries since we only want new ones)
	since := time.Now()
	entries, err := PollDatabase(dbPath, since)
	if err != nil {
		t.Fatalf("PollDatabase() error = %v", err)
	}

	t.Logf("Found %d new entries in Wispr Flow database (expected 0 for recent timestamp)", len(entries))

	// Test polling from a recent timestamp to verify structure if there are any entries
	since = time.Now().Add(-24 * time.Hour)
	entries, err = PollDatabase(dbPath, since)
	if err != nil {
		t.Fatalf("PollDatabase() error = %v", err)
	}

	t.Logf("Found %d entries in last 24 hours", len(entries))

	// Verify structure of first entry if we have any
	if len(entries) > 0 {
		entry := entries[0]
		t.Logf("Sample entry:")
		t.Logf("  ID: %s", entry.TranscriptEntityID)
		t.Logf("  ASR Text: %s", entry.ASRText)
		t.Logf("  Formatted Text: %s", entry.FormattedText)
		t.Logf("  Timestamp: %s", entry.Timestamp)
		t.Logf("  App: %s", entry.App)
		t.Logf("  NumWords: %d", entry.NumWords)

		if entry.TranscriptEntityID == "" {
			t.Error("Expected non-empty TranscriptEntityID")
		}
	}
}

func TestModuleInterface(t *testing.T) {
	m := &Module{}

	if m.Name() != "wisprflow" {
		t.Errorf("Name() = %v, want wisprflow", m.Name())
	}

	if m.Description() == "" {
		t.Error("Description() should not be empty")
	}

	defaultConfig := m.DefaultConfig()
	if defaultConfig == nil {
		t.Error("DefaultConfig() should not be nil")
	}

	cfgMap, ok := defaultConfig.(map[string]interface{})
	if !ok {
		t.Fatal("DefaultConfig() should return map[string]interface{}")
	}

	// Check required config fields
	if _, ok := cfgMap["poll_interval_seconds"]; !ok {
		t.Error("DefaultConfig should contain poll_interval_seconds")
	}
	if _, ok := cfgMap["db_path"]; !ok {
		t.Error("DefaultConfig should contain db_path")
	}
	if _, ok := cfgMap["min_words"]; !ok {
		t.Error("DefaultConfig should contain min_words")
	}
}

func TestValidateConfig(t *testing.T) {
	m := &Module{}

	tests := []struct {
		name    string
		config  interface{}
		wantErr bool
	}{
		{
			name: "valid config",
			config: map[string]interface{}{
				"poll_interval_seconds": 60.0,
				"min_words":             1.0,
			},
			wantErr: false,
		},
		{
			name: "invalid poll interval",
			config: map[string]interface{}{
				"poll_interval_seconds": 0.5,
			},
			wantErr: true,
		},
		{
			name: "negative min_words",
			config: map[string]interface{}{
				"min_words": -1.0,
			},
			wantErr: true,
		},
		{
			name:    "invalid config type",
			config:  "not a map",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
