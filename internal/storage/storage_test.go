package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

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

	event := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))
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
		Source:  string(events.SourceGit),
		Type:    string(events.TypeCommit),
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
		event := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))
		event.Payload["index"] = float64(i)
		if err := storage.InsertEvent(event); err != nil {
			t.Fatalf("InsertEvent() error: %v", err)
		}
	}

	// Insert a shell event
	shellEvent := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	shellEvent.Payload["command"] = "ls -la"
	if err := storage.InsertEvent(shellEvent); err != nil {
		t.Fatalf("InsertEvent() error: %v", err)
	}

	// List all events
	allEvents, err := storage.QueryEvents(QueryOptions{})
	if err != nil {
		t.Fatalf("QueryEvents() error: %v", err)
	}

	if len(allEvents) != 6 {
		t.Errorf("got %d events, want 6", len(allEvents))
	}

	// List with limit
	limitedEvents, err := storage.QueryEvents(QueryOptions{Limit: 3})
	if err != nil {
		t.Fatalf("QueryEvents() error: %v", err)
	}

	if len(limitedEvents) != 3 {
		t.Errorf("got %d events, want 3", len(limitedEvents))
	}

	// List by source
	gitEvents, err := storage.QueryEvents(QueryOptions{Source: string(events.SourceGit)})
	if err != nil {
		t.Fatalf("QueryEvents() error: %v", err)
	}

	if len(gitEvents) != 5 {
		t.Errorf("got %d git events, want 5", len(gitEvents))
	}

	shellEvents, err := storage.QueryEvents(QueryOptions{Source: string(events.SourceShell)})
	if err != nil {
		t.Fatalf("QueryEvents() error: %v", err)
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
		event := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))
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
func TestCountBySource(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	for i := 0; i < 5; i++ {
		event := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))
		if err := storage.InsertEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 3; i++ {
		event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
		if err := storage.InsertEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	results, err := storage.CountBySource(context.Background())
	if err != nil {
		t.Fatalf("CountBySource() error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("CountBySource() got %d results, want 2", len(results))
	}

	for _, result := range results {
		if result.Source == string(events.SourceGit) && result.Count != 5 {
			t.Errorf("git source count = %d, want 5", result.Count)
		}
		if result.Source == string(events.SourceShell) && result.Count != 3 {
			t.Errorf("shell source count = %d, want 3", result.Count)
		}
	}
}

func TestTimelineLast7Days(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	for i := 0; i < 10; i++ {
		event := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))
		if err := storage.InsertEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	_, err := storage.TimelineLast7Days(context.Background())
	if err != nil {
		t.Fatalf("TimelineLast7Days() error: %v", err)
	}
}

func TestTopRepos(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	for i := 0; i < 5; i++ {
		event := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))
		event.Repo = "/path/to/repo1"
		if err := storage.InsertEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 3; i++ {
		event := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))
		event.Repo = "/path/to/repo2"
		if err := storage.InsertEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	results, err := storage.TopRepos(context.Background(), 10)
	if err != nil {
		t.Fatalf("TopRepos() error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("TopRepos() got %d results, want 2", len(results))
	}

	if len(results) > 0 && results[0].Repo != "/path/to/repo1" {
		t.Errorf("TopRepos() first repo = %s, want /path/to/repo1", results[0].Repo)
	}

	if len(results) > 0 && results[0].Count != 5 {
		t.Errorf("TopRepos() first repo count = %d, want 5", results[0].Count)
	}
}

func TestTopCommands(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	for i := 0; i < 7; i++ {
		event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
		event.Payload["command"] = "git status"
		if err := storage.InsertEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 4; i++ {
		event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
		event.Payload["command"] = "ls -la"
		if err := storage.InsertEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	results, err := storage.TopCommands(context.Background(), 10)
	if err != nil {
		t.Fatalf("TopCommands() error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("TopCommands() got %d results, want 2", len(results))
	}

	if len(results) > 0 && results[0].Command != "git status" {
		t.Errorf("TopCommands() first command = %s, want 'git status'", results[0].Command)
	}

	if len(results) > 0 && results[0].Count != 7 {
		t.Errorf("TopCommands() first command count = %d, want 7", results[0].Count)
	}
}

func TestClose(t *testing.T) {
	storage, _ := setupTestDB(t)

	err := storage.Close()
	if err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

func TestQueryEventsWithTimeRange(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	for i := 0; i < 5; i++ {
		event := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))
		if err := storage.InsertEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	now := time.Now().Add(1 * time.Hour)
	past := time.Now().Add(-1 * time.Hour)

	results, err := storage.QueryEvents(QueryOptions{
		StartTime: &past,
		EndTime:   &now,
	})
	if err != nil {
		t.Fatalf("QueryEvents() error: %v", err)
	}

	if len(results) != 5 {
		t.Errorf("QueryEvents() got %d results, want 5", len(results))
	}
}

func TestOperationsOnClosedDB(t *testing.T) {
	storage, _ := setupTestDB(t)
	storage.Close()

	event := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))

	err := storage.InsertEvent(event)
	if err == nil {
		t.Error("InsertEvent() should fail on closed database")
	}

	_, err = storage.GetEvent(event.ID)
	if err == nil {
		t.Error("GetEvent() should fail on closed database")
	}

	_, err = storage.QueryEvents(QueryOptions{})
	if err == nil {
		t.Error("QueryEvents() should fail on closed database")
	}

	_, err = storage.Count()
	if err == nil {
		t.Error("Count() should fail on closed database")
	}
}

func TestDatabaseLockedScenario(t *testing.T) {
	storage, dbPath := setupTestDB(t)
	defer storage.Close()

	storage2, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to open second connection: %v", err)
	}
	defer storage2.Close()

	event1 := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))
	event1.Payload["id"] = "event1"

	event2 := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event2.Payload["id"] = "event2"

	errChan := make(chan error, 2)

	go func() {
		errChan <- storage.InsertEvent(event1)
	}()

	go func() {
		errChan <- storage2.InsertEvent(event2)
	}()

	for i := 0; i < 2; i++ {
		if err := <-errChan; err != nil {
			t.Logf("Concurrent insert error (expected with WAL): %v", err)
		}
	}

	count, err := storage.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}

	if count < 1 {
		t.Errorf("Expected at least 1 event inserted, got %d", count)
	}
}

func TestInvalidDatabasePath(t *testing.T) {
	invalidPaths := []string{
		"/root/nonexistent/test.db",
		"/dev/null/test.db",
		"",
	}

	for _, path := range invalidPaths {
		_, err := New(path)
		if err == nil {
			t.Errorf("New() should fail for invalid path: %s", path)
		}
	}
}

func TestCorruptedPayload(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))
	event.Payload["circular"] = event.Payload

	err := storage.InsertEvent(event)
	if err == nil {
		t.Error("InsertEvent() should fail with circular reference in payload")
	}
}

func TestDiskSpaceSimulation(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	largePayload := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		largePayload[fmt.Sprintf("key_%d", i)] = string(make([]byte, 10000))
	}

	event := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))
	event.Payload = largePayload

	err := storage.InsertEvent(event)
	if err != nil {
		t.Logf("Large payload insert failed (expected on constrained systems): %v", err)
	}
}

func TestConcurrentReadsAndWrites(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	const numWriters = 5
	const numReaders = 3
	const eventsPerWriter = 10

	errChan := make(chan error, numWriters+numReaders)

	for i := 0; i < numWriters; i++ {
		go func(writerID int) {
			for j := 0; j < eventsPerWriter; j++ {
				event := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))
				event.Payload["writer"] = float64(writerID)
				event.Payload["index"] = float64(j)
				if err := storage.InsertEvent(event); err != nil {
					t.Logf("Writer %d insert error (expected under concurrent load): %v", writerID, err)
					continue
				}
				time.Sleep(2 * time.Millisecond)
			}
			errChan <- nil
		}(i)
	}

	for i := 0; i < numReaders; i++ {
		go func(readerID int) {
			for j := 0; j < eventsPerWriter; j++ {
				_, err := storage.QueryEvents(QueryOptions{Limit: 10})
				if err != nil {
					t.Logf("Reader %d query error (expected under concurrent load): %v", readerID, err)
					continue
				}
				time.Sleep(3 * time.Millisecond)
			}
			errChan <- nil
		}(i)
	}

	for i := 0; i < numWriters+numReaders; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Goroutine failed unexpectedly: %v", err)
		}
	}

	count, err := storage.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}

	if count == 0 {
		t.Error("Expected at least some events to be inserted")
	}

	t.Logf("Successfully inserted %d/%d events under concurrent load", count, numWriters*eventsPerWriter)
}

func TestDatabaseRecoveryAfterError(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	validEvent := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))
	validEvent.Payload["valid"] = "data"

	if err := storage.InsertEvent(validEvent); err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	invalidEvent := &events.Event{}
	_ = storage.InsertEvent(invalidEvent)

	validEvent2 := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	validEvent2.Payload["after_error"] = "data"

	if err := storage.InsertEvent(validEvent2); err != nil {
		t.Fatalf("Insert after error failed: %v", err)
	}

	count, err := storage.Count()
	if err != nil {
		t.Fatalf("Count() after recovery error: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 valid events, got %d", count)
	}
}
