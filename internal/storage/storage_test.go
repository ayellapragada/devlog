package storage

import (
	"os"
	"path/filepath"
	"testing"

	"devlog/internal/events"
)

func setupTestDB(t *testing.T) (*Storage, string) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB() error: %v", err)
	}

	storage, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	return storage, dbPath
}

func TestInitDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB() error: %v", err)
	}

	// Check file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}

	// Try to initialize again (should fail)
	err = InitDB(dbPath)
	if err == nil {
		t.Error("InitDB() should fail when database already exists")
	}
}

func TestNewStorage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Try to open non-existent database
	_, err := New(dbPath)
	if err == nil {
		t.Error("New() should fail when database doesn't exist")
	}

	// Create database
	if err := InitDB(dbPath); err != nil {
		t.Fatal(err)
	}

	// Now it should work
	storage, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer storage.Close()
}

func TestInsertAndGetEvent(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event := events.NewEvent(events.SourceGit, events.TypeCommit)
	event.Repo = "/path/to/repo"
	event.Branch = "main"
	event.Payload["hash"] = "abc123"
	event.Payload["message"] = "Test commit"

	// Insert event
	err := storage.InsertEvent(event)
	if err != nil {
		t.Fatalf("InsertEvent() error: %v", err)
	}

	// Retrieve event
	retrieved, err := storage.GetEvent(event.ID)
	if err != nil {
		t.Fatalf("GetEvent() error: %v", err)
	}

	// Compare
	if retrieved.ID != event.ID {
		t.Errorf("got ID %s, want %s", retrieved.ID, event.ID)
	}

	if retrieved.Source != event.Source {
		t.Errorf("got source %s, want %s", retrieved.Source, event.Source)
	}

	if retrieved.Type != event.Type {
		t.Errorf("got type %s, want %s", retrieved.Type, event.Type)
	}

	if retrieved.Repo != event.Repo {
		t.Errorf("got repo %s, want %s", retrieved.Repo, event.Repo)
	}

	if retrieved.Branch != event.Branch {
		t.Errorf("got branch %s, want %s", retrieved.Branch, event.Branch)
	}

	if retrieved.Payload["hash"] != event.Payload["hash"] {
		t.Errorf("got hash %v, want %v", retrieved.Payload["hash"], event.Payload["hash"])
	}
}

func TestInsertInvalidEvent(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	// Create invalid event (missing required field)
	event := &events.Event{
		Version: 1,
		ID:      "",
		Source:  events.SourceGit,
		Type:    events.TypeCommit,
		Payload: map[string]interface{}{},
	}

	err := storage.InsertEvent(event)
	if err == nil {
		t.Error("InsertEvent() should fail for invalid event")
	}
}

func TestGetNonExistentEvent(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	_, err := storage.GetEvent("non-existent-id")
	if err == nil {
		t.Error("GetEvent() should fail for non-existent event")
	}
}

func TestListEvents(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	// Insert multiple events
	for i := 0; i < 5; i++ {
		event := events.NewEvent(events.SourceGit, events.TypeCommit)
		event.Payload["index"] = float64(i)
		if err := storage.InsertEvent(event); err != nil {
			t.Fatalf("InsertEvent() error: %v", err)
		}
	}

	// Insert a shell event
	shellEvent := events.NewEvent(events.SourceShell, events.TypeCommand)
	shellEvent.Payload["command"] = "ls -la"
	if err := storage.InsertEvent(shellEvent); err != nil {
		t.Fatalf("InsertEvent() error: %v", err)
	}

	// List all events
	allEvents, err := storage.ListEvents(0, "")
	if err != nil {
		t.Fatalf("ListEvents() error: %v", err)
	}

	if len(allEvents) != 6 {
		t.Errorf("got %d events, want 6", len(allEvents))
	}

	// List with limit
	limitedEvents, err := storage.ListEvents(3, "")
	if err != nil {
		t.Fatalf("ListEvents() error: %v", err)
	}

	if len(limitedEvents) != 3 {
		t.Errorf("got %d events, want 3", len(limitedEvents))
	}

	// List by source
	gitEvents, err := storage.ListEvents(0, events.SourceGit)
	if err != nil {
		t.Fatalf("ListEvents() error: %v", err)
	}

	if len(gitEvents) != 5 {
		t.Errorf("got %d git events, want 5", len(gitEvents))
	}

	shellEvents, err := storage.ListEvents(0, events.SourceShell)
	if err != nil {
		t.Fatalf("ListEvents() error: %v", err)
	}

	if len(shellEvents) != 1 {
		t.Errorf("got %d shell events, want 1", len(shellEvents))
	}
}

func TestCount(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	// Initially empty
	count, err := storage.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}

	if count != 0 {
		t.Errorf("got count %d, want 0", count)
	}

	// Insert events
	for i := 0; i < 10; i++ {
		event := events.NewEvent(events.SourceGit, events.TypeCommit)
		if err := storage.InsertEvent(event); err != nil {
			t.Fatalf("InsertEvent() error: %v", err)
		}
	}

	// Check count
	count, err = storage.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}

	if count != 10 {
		t.Errorf("got count %d, want 10", count)
	}
}
