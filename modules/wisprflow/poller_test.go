package wisprflow

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNewPoller(t *testing.T) {
	dbPath := "/path/to/db"
	dataDir := "/path/to/data"
	interval := 30 * time.Second
	minWords := 5

	poller := NewPoller(dbPath, dataDir, interval, minWords)

	if poller.dbPath != dbPath {
		t.Errorf("Expected dbPath %s, got %s", dbPath, poller.dbPath)
	}

	if poller.dataDir != dataDir {
		t.Errorf("Expected dataDir %s, got %s", dataDir, poller.dataDir)
	}

	if poller.pollInterval != interval {
		t.Errorf("Expected interval %v, got %v", interval, poller.pollInterval)
	}

	if poller.minWords != minWords {
		t.Errorf("Expected minWords %d, got %d", minWords, poller.minWords)
	}
}

func TestPollerName(t *testing.T) {
	poller := NewPoller("", "", time.Second, 0)

	if poller.Name() != "wisprflow" {
		t.Errorf("Expected name 'wisprflow', got '%s'", poller.Name())
	}
}

func TestPollerPollInterval(t *testing.T) {
	interval := 45 * time.Second
	poller := NewPoller("", "", interval, 0)

	if poller.PollInterval() != interval {
		t.Errorf("Expected interval %v, got %v", interval, poller.PollInterval())
	}
}

func TestPollerPollWithNonexistentDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "nonexistent.db")
	dataDir := t.TempDir()

	poller := NewPoller(dbPath, dataDir, time.Second, 0)

	_, err := poller.Poll()
	if err == nil {
		t.Error("Expected error when polling nonexistent database")
	}
}

func TestPollerPollWithMinWordsFiltering(t *testing.T) {
	t.Skip("Skipping integration test - requires real Wispr Flow database")
}

func TestPollerShouldRetryOnError(t *testing.T) {
	poller := NewPoller("", "", time.Second, 0)

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
