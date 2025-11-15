package testutil

import (
	"context"
	"path/filepath"
	"testing"

	"devlog/internal/config"
	"devlog/internal/events"
	"devlog/internal/logger"
	"devlog/internal/storage"
)

func NewTestStorage(t *testing.T) *storage.Storage {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	if err := storage.InitDB(dbPath); err != nil {
		t.Fatalf("init test db: %v", err)
	}

	store, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
	})

	return store
}

func NewTestConfig() *config.Config {
	return &config.Config{
		ObsidianPath:      "/tmp/obsidian",
		HTTP:              config.HTTPConfig{Port: 8573},
		SessionGapMinutes: 45,
		Modules:           make(map[string]config.ModuleConfig),
	}
}

func NewTestLogger() *logger.Logger {
	return logger.Default()
}

func NewTestEvent(source, eventType string) *events.Event {
	event := events.NewEvent(source, eventType)
	event.Repo = "test/repo"
	event.Branch = "main"
	event.Payload = map[string]interface{}{
		"test": "data",
	}
	return event
}

func MustInsertEvents(t *testing.T, store *storage.Storage, evts ...*events.Event) {
	t.Helper()
	ctx := context.Background()
	for _, event := range evts {
		if err := store.InsertEventContext(ctx, event); err != nil {
			t.Fatalf("insert test event: %v", err)
		}
	}
}

func AssertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

func AssertError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error but got nil", msg)
	}
}

func AssertEqual(t *testing.T, got, want interface{}, msg string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %v, want %v", msg, got, want)
	}
}
