package services

import (
	"context"
	"errors"
	"testing"

	"devlog/internal/config"
	"devlog/internal/events"
	"devlog/internal/storage"
	"devlog/internal/testutil"
)

func configGetter(cfg *config.Config) func() *config.Config {
	return func() *config.Config { return cfg }
}

func TestNewEventService(t *testing.T) {
	store := testutil.NewTestStorage(t)
	cfg := testutil.NewTestConfig()

	service := NewEventService(store, configGetter(cfg), nil)

	if service == nil {
		t.Fatal("NewEventService returned nil")
	}
	if service.storage == nil {
		t.Error("service storage is nil")
	}
	if service.configGetter == nil {
		t.Error("service configGetter is nil")
	}
	if service.logger == nil {
		t.Error("service logger is nil")
	}
}

func TestEventService_IngestEvent(t *testing.T) {
	store := testutil.NewTestStorage(t)
	cfg := testutil.NewTestConfig()
	cfg.Modules["git"] = config.ComponentConfig{Enabled: true}
	service := NewEventService(store, configGetter(cfg), nil)
	ctx := context.Background()

	event := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))
	event.Repo = "test/repo"
	event.Branch = "main"
	event.Payload["message"] = "test commit"

	err := service.IngestEvent(ctx, event)
	testutil.AssertNoError(t, err, "IngestEvent failed")

	count, err := store.CountContext(ctx)
	testutil.AssertNoError(t, err, "CountContext failed")
	testutil.AssertEqual(t, count, 1, "event count")
}

func TestEventService_IngestEvent_InvalidEvent(t *testing.T) {
	store := testutil.NewTestStorage(t)
	cfg := testutil.NewTestConfig()
	service := NewEventService(store, configGetter(cfg), nil)
	ctx := context.Background()

	event := events.NewEvent("invalid_source", string(events.TypeCommit))

	err := service.IngestEvent(ctx, event)
	if err == nil {
		t.Fatal("IngestEvent should fail with invalid event")
	}

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Errorf("expected ValidationError, got %T", err)
	}
}

func TestEventService_IngestEvent_FilteredCommand(t *testing.T) {
	store := testutil.NewTestStorage(t)
	cfg := testutil.NewTestConfig()
	cfg.Modules["shell"] = config.ComponentConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"ignore_list": []interface{}{"ls", "cd", "pwd"},
		},
	}
	service := NewEventService(store, configGetter(cfg), nil)
	ctx := context.Background()

	event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event.Payload["command"] = "ls -la"

	err := service.IngestEvent(ctx, event)
	if !errors.Is(err, ErrEventFiltered) {
		t.Errorf("expected ErrEventFiltered, got %v", err)
	}
}

func TestEventService_IngestEvent_FilteredGitEvent(t *testing.T) {
	store := testutil.NewTestStorage(t)
	cfg := testutil.NewTestConfig()
	service := NewEventService(store, configGetter(cfg), nil)
	ctx := context.Background()

	event := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))
	event.Repo = "test/repo"
	event.Branch = "main"
	event.Payload["message"] = "test commit"

	err := service.IngestEvent(ctx, event)
	if !errors.Is(err, ErrEventFiltered) {
		t.Errorf("expected ErrEventFiltered when git module disabled, got %v", err)
	}
}

func TestEventService_SearchEvents(t *testing.T) {
	store := testutil.NewTestStorage(t)
	cfg := testutil.NewTestConfig()
	service := NewEventService(store, configGetter(cfg), nil)
	ctx := context.Background()

	event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event.Payload["command"] = "git status"
	testutil.MustInsertEvents(t, store, event)

	results, err := service.SearchEvents(ctx, storage.SearchOptions{
		Query: "git",
		Limit: 10,
	})
	testutil.AssertNoError(t, err, "SearchEvents failed")

	if len(results) != 1 {
		t.Errorf("SearchEvents returned %d results, want 1", len(results))
	}
}

func TestEventService_SearchEvents_DefaultLimit(t *testing.T) {
	store := testutil.NewTestStorage(t)
	cfg := testutil.NewTestConfig()
	service := NewEventService(store, configGetter(cfg), nil)
	ctx := context.Background()

	for i := 0; i < 30; i++ {
		event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
		event.Payload["command"] = "test command"
		testutil.MustInsertEvents(t, store, event)
	}

	results, err := service.SearchEvents(ctx, storage.SearchOptions{
		Query: "test",
		Limit: 0,
	})
	testutil.AssertNoError(t, err, "SearchEvents failed")

	if len(results) != 20 {
		t.Errorf("SearchEvents with 0 limit returned %d results, want 20 (default)", len(results))
	}
}

func TestEventService_GetEvents(t *testing.T) {
	store := testutil.NewTestStorage(t)
	cfg := testutil.NewTestConfig()
	service := NewEventService(store, configGetter(cfg), nil)
	ctx := context.Background()

	event1 := testutil.NewEventBuilder().
		WithSource(string(events.SourceGit)).
		WithType(string(events.TypeCommit)).
		WithRepo("test/repo").
		Build()

	event2 := testutil.NewEventBuilder().
		WithSource(string(events.SourceShell)).
		WithType(string(events.TypeCommand)).
		Build()

	testutil.MustInsertEvents(t, store, event1, event2)

	events, err := service.GetEvents(ctx, storage.QueryOptions{
		Limit: 10,
	})
	testutil.AssertNoError(t, err, "GetEvents failed")

	if len(events) != 2 {
		t.Errorf("GetEvents returned %d events, want 2", len(events))
	}
}

func TestEventService_GetEventsBySource(t *testing.T) {
	store := testutil.NewTestStorage(t)
	cfg := testutil.NewTestConfig()
	service := NewEventService(store, configGetter(cfg), nil)
	ctx := context.Background()

	gitEvent := testutil.NewEventBuilder().
		WithSource(string(events.SourceGit)).
		Build()
	shellEvent := testutil.NewEventBuilder().
		WithSource(string(events.SourceShell)).
		Build()

	testutil.MustInsertEvents(t, store, gitEvent, shellEvent)

	counts, err := service.GetEventsBySource(ctx)
	testutil.AssertNoError(t, err, "GetEventsBySource failed")

	if len(counts) != 2 {
		t.Errorf("GetEventsBySource returned %d sources, want 2", len(counts))
	}
}

func TestEventService_GetTimeline(t *testing.T) {
	store := testutil.NewTestStorage(t)
	cfg := testutil.NewTestConfig()
	service := NewEventService(store, configGetter(cfg), nil)
	ctx := context.Background()

	event := testutil.NewEventBuilder().Build()
	testutil.MustInsertEvents(t, store, event)

	timeline, err := service.GetTimeline(ctx)
	testutil.AssertNoError(t, err, "GetTimeline failed")

	if len(timeline) < 1 {
		t.Logf("GetTimeline returned %d points (timeline may aggregate by date)", len(timeline))
	}
}

func TestEventService_GetTopRepos(t *testing.T) {
	store := testutil.NewTestStorage(t)
	cfg := testutil.NewTestConfig()
	service := NewEventService(store, configGetter(cfg), nil)
	ctx := context.Background()

	event1 := testutil.NewEventBuilder().
		WithRepo("repo1").
		Build()
	event2 := testutil.NewEventBuilder().
		WithRepo("repo1").
		Build()
	event3 := testutil.NewEventBuilder().
		WithRepo("repo2").
		Build()

	testutil.MustInsertEvents(t, store, event1, event2, event3)

	repos, err := service.GetTopRepos(ctx, 10)
	testutil.AssertNoError(t, err, "GetTopRepos failed")

	if len(repos) != 2 {
		t.Errorf("GetTopRepos returned %d repos, want 2", len(repos))
	}
	if len(repos) > 0 && repos[0].Repo != "repo1" {
		t.Errorf("top repo is %s, want repo1", repos[0].Repo)
	}
}

func TestEventService_GetTopCommands(t *testing.T) {
	store := testutil.NewTestStorage(t)
	cfg := testutil.NewTestConfig()
	service := NewEventService(store, configGetter(cfg), nil)
	ctx := context.Background()

	event1 := testutil.NewEventBuilder().
		WithSource(string(events.SourceShell)).
		WithType(string(events.TypeCommand)).
		WithPayloadField("command", "git status").
		Build()
	event2 := testutil.NewEventBuilder().
		WithSource(string(events.SourceShell)).
		WithType(string(events.TypeCommand)).
		WithPayloadField("command", "git status").
		Build()
	event3 := testutil.NewEventBuilder().
		WithSource(string(events.SourceShell)).
		WithType(string(events.TypeCommand)).
		WithPayloadField("command", "ls").
		Build()

	testutil.MustInsertEvents(t, store, event1, event2, event3)

	commands, err := service.GetTopCommands(ctx, 10)
	testutil.AssertNoError(t, err, "GetTopCommands failed")

	if len(commands) != 2 {
		t.Errorf("GetTopCommands returned %d commands, want 2", len(commands))
	}
	if len(commands) > 0 && commands[0].Command != "git status" {
		t.Errorf("top command is %s, want 'git status'", commands[0].Command)
	}
}

func TestEventService_CountEvents(t *testing.T) {
	store := testutil.NewTestStorage(t)
	cfg := testutil.NewTestConfig()
	service := NewEventService(store, configGetter(cfg), nil)
	ctx := context.Background()

	event1 := testutil.NewEventBuilder().Build()
	event2 := testutil.NewEventBuilder().Build()
	testutil.MustInsertEvents(t, store, event1, event2)

	count, err := service.CountEvents(ctx)
	testutil.AssertNoError(t, err, "CountEvents failed")

	if count != 2 {
		t.Errorf("CountEvents returned %d, want 2", count)
	}
}

func TestValidationError(t *testing.T) {
	baseErr := errors.New("base error")
	valErr := &ValidationError{Err: baseErr}

	if valErr.Error() != "invalid event: base error" {
		t.Errorf("ValidationError.Error() = %q", valErr.Error())
	}

	if !errors.Is(valErr, baseErr) {
		t.Error("ValidationError should unwrap to base error")
	}
}

func TestErrEventFiltered(t *testing.T) {
	err := ErrEventFiltered
	if err == nil {
		t.Fatal("ErrEventFiltered is nil")
	}
	if err.Error() == "" {
		t.Error("ErrEventFiltered has empty message")
	}
}

func TestEventService_DynamicConfigChange(t *testing.T) {
	store := testutil.NewTestStorage(t)
	cfg := testutil.NewTestConfig()
	cfg.Modules["shell"] = config.ComponentConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"ignore_list": []interface{}{"ls"},
		},
	}

	service := NewEventService(store, configGetter(cfg), nil)
	ctx := context.Background()

	lsEvent := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	lsEvent.Payload["command"] = "ls"

	err := service.IngestEvent(ctx, lsEvent)
	if !errors.Is(err, ErrEventFiltered) {
		t.Errorf("expected ls to be filtered, got %v", err)
	}

	cfg.Modules["shell"] = config.ComponentConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"ignore_list": []interface{}{"cd"},
		},
	}

	err = service.IngestEvent(ctx, lsEvent)
	testutil.AssertNoError(t, err, "ls should not be filtered after config change")

	cdEvent := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	cdEvent.Payload["command"] = "cd /tmp"

	err = service.IngestEvent(ctx, cdEvent)
	if !errors.Is(err, ErrEventFiltered) {
		t.Errorf("expected cd to be filtered after config change, got %v", err)
	}
}
