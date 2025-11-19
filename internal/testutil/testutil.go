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
		HTTP:    config.HTTPConfig{Port: 8573},
		Modules: make(map[string]config.ComponentConfig),
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

type EventBuilder struct {
	event *events.Event
}

func NewEventBuilder() *EventBuilder {
	return &EventBuilder{
		event: events.NewEvent(string(events.SourceGit), string(events.TypeCommit)),
	}
}

func (b *EventBuilder) WithSource(source string) *EventBuilder {
	b.event.Source = source
	return b
}

func (b *EventBuilder) WithType(eventType string) *EventBuilder {
	b.event.Type = eventType
	return b
}

func (b *EventBuilder) WithRepo(repo string) *EventBuilder {
	b.event.Repo = repo
	return b
}

func (b *EventBuilder) WithBranch(branch string) *EventBuilder {
	b.event.Branch = branch
	return b
}

func (b *EventBuilder) WithPayload(payload map[string]interface{}) *EventBuilder {
	b.event.Payload = payload
	return b
}

func (b *EventBuilder) WithPayloadField(key string, value interface{}) *EventBuilder {
	if b.event.Payload == nil {
		b.event.Payload = make(map[string]interface{})
	}
	b.event.Payload[key] = value
	return b
}

func (b *EventBuilder) Build() *events.Event {
	return b.event
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

func AssertGreaterThan(t *testing.T, got, min int, msg string) {
	t.Helper()
	if got <= min {
		t.Fatalf("%s: got %d, want > %d", msg, got, min)
	}
}

func AssertContains(t *testing.T, haystack, needle string, msg string) {
	t.Helper()
	if haystack == "" || needle == "" {
		t.Fatalf("%s: empty string provided", msg)
	}
	if !contains(haystack, needle) {
		t.Fatalf("%s: %q does not contain %q", msg, haystack, needle)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(haystack) > len(needle) && hasSubstring(haystack, needle))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
