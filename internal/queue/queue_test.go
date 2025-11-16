package queue

import (
	"os"
	"path/filepath"
	"testing"

	"devlog/internal/events"
)

func TestNew(t *testing.T) {
	tmpDir := t.TempDir()
	queueDir := filepath.Join(tmpDir, "queue")

	q, err := New(queueDir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if q == nil {
		t.Fatal("New() returned nil queue")
	}

	if _, err := os.Stat(queueDir); os.IsNotExist(err) {
		t.Error("queue directory was not created")
	}
}

func TestEnqueueAndList(t *testing.T) {
	tmpDir := t.TempDir()
	q, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	event1 := events.NewEvent(events.SourceGit, events.TypeCommit)
	event1.Payload["message"] = "test commit 1"

	event2 := events.NewEvent(events.SourceShell, events.TypeCommand)
	event2.Payload["command"] = "ls -la"

	if err := q.Enqueue(event1); err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}

	if err := q.Enqueue(event2); err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}

	queued, err := q.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(queued) != 2 {
		t.Fatalf("got %d events, want 2", len(queued))
	}

	if queued[0].ID != event1.ID {
		t.Errorf("first event ID mismatch: got %s, want %s", queued[0].ID, event1.ID)
	}

	if queued[1].ID != event2.ID {
		t.Errorf("second event ID mismatch: got %s, want %s", queued[1].ID, event2.ID)
	}
}

func TestRemove(t *testing.T) {
	tmpDir := t.TempDir()
	q, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	event := events.NewEvent(events.SourceGit, events.TypeCommit)
	if err := q.Enqueue(event); err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}

	queued, err := q.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(queued) != 1 {
		t.Fatalf("got %d events before remove, want 1", len(queued))
	}

	if err := q.Remove(event.ID); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	queued, err = q.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(queued) != 0 {
		t.Errorf("got %d events after remove, want 0", len(queued))
	}
}

func TestRemoveNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	q, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	err = q.Remove("non-existent-id")
	if err == nil {
		t.Error("Remove() should fail for non-existent event")
	}
}

func TestCount(t *testing.T) {
	tmpDir := t.TempDir()
	q, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	count, err := q.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 0 {
		t.Errorf("got count %d, want 0", count)
	}

	for i := 0; i < 5; i++ {
		event := events.NewEvent(events.SourceGit, events.TypeCommit)
		if err := q.Enqueue(event); err != nil {
			t.Fatalf("Enqueue() error: %v", err)
		}
	}

	count, err = q.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 5 {
		t.Errorf("got count %d, want 5", count)
	}
}

func TestClear(t *testing.T) {
	tmpDir := t.TempDir()
	q, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	for i := 0; i < 3; i++ {
		event := events.NewEvent(events.SourceGit, events.TypeCommit)
		if err := q.Enqueue(event); err != nil {
			t.Fatalf("Enqueue() error: %v", err)
		}
	}

	count, _ := q.Count()
	if count != 3 {
		t.Fatalf("got count %d before clear, want 3", count)
	}

	if err := q.Clear(); err != nil {
		t.Fatalf("Clear() error: %v", err)
	}

	count, err = q.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 0 {
		t.Errorf("got count %d after clear, want 0", count)
	}
}

func TestListIgnoresInvalidFiles(t *testing.T) {
	tmpDir := t.TempDir()
	q, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	invalidPath := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidPath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	txtPath := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(txtPath, []byte("text file"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	queued, err := q.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(queued) != 0 {
		t.Errorf("got %d events, want 0 (should ignore invalid files)", len(queued))
	}
}
