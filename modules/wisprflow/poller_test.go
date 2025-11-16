package wisprflow

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestNewPoller(t *testing.T) {
	dbPath := "/path/to/db"
	dataDir := t.TempDir()
	interval := 30 * time.Second
	minWords := 5

	poller, err := NewPoller(dbPath, dataDir, interval, minWords)
	if err != nil {
		t.Fatalf("Failed to create poller: %v", err)
	}

	if poller.dbPath != dbPath {
		t.Errorf("Expected dbPath %s, got %s", dbPath, poller.dbPath)
	}

	if poller.pollInterval != interval {
		t.Errorf("Expected interval %v, got %v", interval, poller.pollInterval)
	}

	if poller.minWords != minWords {
		t.Errorf("Expected minWords %d, got %d", minWords, poller.minWords)
	}
}

func TestPollerName(t *testing.T) {
	dataDir := t.TempDir()
	poller, err := NewPoller("", dataDir, time.Second, 0)
	if err != nil {
		t.Fatalf("Failed to create poller: %v", err)
	}

	if poller.Name() != "wisprflow" {
		t.Errorf("Expected name 'wisprflow', got '%s'", poller.Name())
	}
}

func TestPollerPollInterval(t *testing.T) {
	dataDir := t.TempDir()
	interval := 45 * time.Second
	poller, err := NewPoller("", dataDir, interval, 0)
	if err != nil {
		t.Fatalf("Failed to create poller: %v", err)
	}

	if poller.PollInterval() != interval {
		t.Errorf("Expected interval %v, got %v", interval, poller.PollInterval())
	}
}

func TestPollerPollWithNonexistentDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "nonexistent.db")
	dataDir := t.TempDir()

	poller, err := NewPoller(dbPath, dataDir, time.Second, 0)
	if err != nil {
		t.Fatalf("Failed to create poller: %v", err)
	}

	_, err = poller.Poll(context.Background())
	if err == nil {
		t.Error("Expected error when polling nonexistent database")
	}
}

func TestPollerPollWithMinWordsFiltering(t *testing.T) {
	t.Skip("Skipping integration test - requires real Wispr Flow database")
}

func TestPollerShouldRetryOnError(t *testing.T) {
	dataDir := t.TempDir()
	poller, err := NewPoller("", dataDir, time.Second, 0)
	if err != nil {
		t.Fatalf("Failed to create poller: %v", err)
	}

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "UNIQUE constraint error should not retry",
			err:      &testError{msg: "UNIQUE constraint failed"},
			expected: false,
		},
		{
			name:     "Other error should retry",
			err:      &testError{msg: "some other error"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := poller.ShouldRetryOnError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
