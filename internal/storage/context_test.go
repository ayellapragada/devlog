package storage

import (
	"context"
	"testing"
	"time"

	"devlog/internal/events"
)

func TestInsertEventContext(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event := events.NewEvent(events.SourceGit, events.TypeCommit)
	event.Repo = "test/repo"
	event.Branch = "main"
	event.Payload["test"] = "data"

	ctx := context.Background()
	err := storage.InsertEventContext(ctx, event)
	if err != nil {
		t.Fatalf("InsertEventContext() error: %v", err)
	}

	retrieved, err := storage.GetEventContext(ctx, event.ID)
	if err != nil {
		t.Fatalf("GetEventContext() error: %v", err)
	}

	if retrieved.ID != event.ID {
		t.Errorf("got ID %s, want %s", retrieved.ID, event.ID)
	}
}

func TestInsertEventContextTimeout(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event := events.NewEvent(events.SourceGit, events.TypeCommit)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond)

	err := storage.InsertEventContext(ctx, event)
	if err == nil {
		t.Error("InsertEventContext() should fail with expired context")
	}
}

func TestGetEventContext(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event := events.NewEvent(events.SourceGit, events.TypeCommit)
	if err := storage.InsertEvent(event); err != nil {
		t.Fatalf("InsertEvent() error: %v", err)
	}

	ctx := context.Background()
	retrieved, err := storage.GetEventContext(ctx, event.ID)
	if err != nil {
		t.Fatalf("GetEventContext() error: %v", err)
	}

	if retrieved.ID != event.ID {
		t.Errorf("got ID %s, want %s", retrieved.ID, event.ID)
	}
}

func TestGetEventContextCancelled(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := storage.GetEventContext(ctx, "some-id")
	if err == nil {
		t.Error("GetEventContext() should fail with cancelled context")
	}
}

func TestListEventsContext(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	for i := 0; i < 3; i++ {
		event := events.NewEvent(events.SourceGit, events.TypeCommit)
		if err := storage.InsertEvent(event); err != nil {
			t.Fatalf("InsertEvent() error: %v", err)
		}
	}

	ctx := context.Background()
	events, err := storage.ListEventsContext(ctx, 0, "")
	if err != nil {
		t.Fatalf("ListEventsContext() error: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("got %d events, want 3", len(events))
	}
}

func TestListEventsContextWithFilters(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	for i := 0; i < 5; i++ {
		event := events.NewEvent(events.SourceGit, events.TypeCommit)
		if err := storage.InsertEvent(event); err != nil {
			t.Fatalf("InsertEvent() error: %v", err)
		}
	}

	shellEvent := events.NewEvent(events.SourceShell, events.TypeCommand)
	if err := storage.InsertEvent(shellEvent); err != nil {
		t.Fatalf("InsertEvent() error: %v", err)
	}

	ctx := context.Background()

	gitEvents, err := storage.ListEventsContext(ctx, 0, events.SourceGit)
	if err != nil {
		t.Fatalf("ListEventsContext() error: %v", err)
	}
	if len(gitEvents) != 5 {
		t.Errorf("got %d git events, want 5", len(gitEvents))
	}

	limitedEvents, err := storage.ListEventsContext(ctx, 2, "")
	if err != nil {
		t.Fatalf("ListEventsContext() error: %v", err)
	}
	if len(limitedEvents) != 2 {
		t.Errorf("got %d limited events, want 2", len(limitedEvents))
	}
}

func TestCountContext(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	ctx := context.Background()

	count, err := storage.CountContext(ctx)
	if err != nil {
		t.Fatalf("CountContext() error: %v", err)
	}
	if count != 0 {
		t.Errorf("got count %d, want 0", count)
	}

	for i := 0; i < 7; i++ {
		event := events.NewEvent(events.SourceGit, events.TypeCommit)
		if err := storage.InsertEvent(event); err != nil {
			t.Fatalf("InsertEvent() error: %v", err)
		}
	}

	count, err = storage.CountContext(ctx)
	if err != nil {
		t.Fatalf("CountContext() error: %v", err)
	}
	if count != 7 {
		t.Errorf("got count %d, want 7", count)
	}
}

func TestCountContextCancelled(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := storage.CountContext(ctx)
	if err == nil {
		t.Error("CountContext() should fail with cancelled context")
	}
}
