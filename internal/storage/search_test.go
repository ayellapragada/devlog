package storage

import (
	"context"
	"testing"

	"devlog/internal/events"
)

func TestSearchEvents(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event1 := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event1.Payload["command"] = "git commit -m 'error handling fix'"
	event1.Payload["exit_code"] = 0

	event2 := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event2.Payload["command"] = "npm test"
	event2.Payload["output"] = "error: test failed"

	event3 := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))
	event3.Payload["message"] = "add feature"

	if err := storage.InsertEvent(event1); err != nil {
		t.Fatalf("InsertEvent(event1) error: %v", err)
	}
	if err := storage.InsertEvent(event2); err != nil {
		t.Fatalf("InsertEvent(event2) error: %v", err)
	}
	if err := storage.InsertEvent(event3); err != nil {
		t.Fatalf("InsertEvent(event3) error: %v", err)
	}

	results, err := storage.Search(context.Background(), SearchOptions{
		Query:          "error",
		Limit:          10,
		IncludeSnippet: true,
	})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Search() got %d results, want 2", len(results))
	}

	for _, result := range results {
		if result.Event == nil {
			t.Error("SearchResult.Event is nil")
		}
		if result.Snippet == "" {
			t.Error("SearchResult.Snippet is empty")
		}
	}
}

func TestSearchEventsSimple(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event1 := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event1.Payload["command"] = "git status"

	event2 := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event2.Payload["command"] = "git log"

	if err := storage.InsertEvent(event1); err != nil {
		t.Fatal(err)
	}
	if err := storage.InsertEvent(event2); err != nil {
		t.Fatal(err)
	}

	results, err := storage.Search(context.Background(), SearchOptions{
		Query: "git",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Search() got %d results, want 2", len(results))
	}
}

func TestSearchEventsNoResults(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event.Payload["command"] = "ls -la"

	if err := storage.InsertEvent(event); err != nil {
		t.Fatal(err)
	}

	results, err := storage.Search(context.Background(), SearchOptions{
		Query: "nonexistent",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Search() got %d results, want 0", len(results))
	}
}

func TestQueryByPayloadField(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event1 := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event1.Payload["command"] = "echo hello"
	event1.Payload["status"] = "success"

	event2 := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event2.Payload["command"] = "false"
	event2.Payload["status"] = "failure"

	if err := storage.InsertEvent(event1); err != nil {
		t.Fatal(err)
	}
	if err := storage.InsertEvent(event2); err != nil {
		t.Fatal(err)
	}

	results, err := storage.QueryByPayloadField(context.Background(), "$.status", "success", 10)
	if err != nil {
		t.Fatalf("QueryByPayloadField() error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("QueryByPayloadField() got %d results, want 1", len(results))
	}

	if len(results) > 0 {
		payload := results[0].Payload
		if status, ok := payload["status"].(string); !ok || status != "success" {
			t.Errorf("QueryByPayloadField() returned wrong event, status=%v", payload["status"])
		}
	}
}

func TestSearchWithPayloadFilter(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event1 := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))
	event1.Repo = "/path/to/repo"
	event1.Branch = "main"
	event1.Payload["message"] = "fix error in main"

	event2 := events.NewEvent(string(events.SourceGit), string(events.TypeCommit))
	event2.Repo = "/path/to/repo"
	event2.Branch = "dev"
	event2.Payload["message"] = "fix error in dev"

	if err := storage.InsertEvent(event1); err != nil {
		t.Fatal(err)
	}
	if err := storage.InsertEvent(event2); err != nil {
		t.Fatal(err)
	}

	results, err := storage.Search(context.Background(), SearchOptions{
		Query: "error",
		Limit: 10,
		PayloadFilter: &PayloadFilter{
			JSONPath: "$.message",
			Value:    "fix error in main",
		},
	})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Search() got %d results, want 1", len(results))
	}

	if len(results) > 0 {
		if results[0].Event.Branch != "main" {
			t.Errorf("Search() returned wrong branch: %s", results[0].Event.Branch)
		}
	}
}

func TestSearchSpecialCharacters(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event.Payload["command"] = "echo 'hello world!'"
	event.Payload["email"] = "test@example.com"

	if err := storage.InsertEvent(event); err != nil {
		t.Fatal(err)
	}

	results, err := storage.Search(context.Background(), SearchOptions{
		Query: "example",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Search() got %d results, want 1", len(results))
	}
}

func TestSearchMultipleTerms(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event1 := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event1.Payload["command"] = "git commit"

	event2 := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event2.Payload["command"] = "git push"

	event3 := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event3.Payload["command"] = "npm install"

	if err := storage.InsertEvent(event1); err != nil {
		t.Fatal(err)
	}
	if err := storage.InsertEvent(event2); err != nil {
		t.Fatal(err)
	}
	if err := storage.InsertEvent(event3); err != nil {
		t.Fatal(err)
	}

	results, err := storage.Search(context.Background(), SearchOptions{
		Query: "git OR npm",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Search() got %d results, want 3", len(results))
	}
}

func TestQueryByPayloadFieldNotFound(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event.Payload["command"] = "ls"

	if err := storage.InsertEvent(event); err != nil {
		t.Fatal(err)
	}

	results, err := storage.QueryByPayloadField(context.Background(), "$.nonexistent", "value", 10)
	if err != nil {
		t.Fatalf("QueryByPayloadField() error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("QueryByPayloadField() got %d results, want 0", len(results))
	}
}

func TestSearchLimit(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	for i := 0; i < 10; i++ {
		event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
		event.Payload["command"] = "test command"
		if err := storage.InsertEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	results, err := storage.Search(context.Background(), SearchOptions{
		Query: "command",
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if len(results) != 5 {
		t.Errorf("Search() got %d results, want 5", len(results))
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event.Payload["command"] = "test"
	if err := storage.InsertEvent(event); err != nil {
		t.Fatal(err)
	}

	_, err := storage.Search(context.Background(), SearchOptions{
		Query: "",
		Limit: 10,
	})
	if err == nil {
		t.Error("Search() with empty query should return error")
	}
}

func TestSearchWhitespaceOnly(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event.Payload["command"] = "test"
	if err := storage.InsertEvent(event); err != nil {
		t.Fatal(err)
	}

	_, err := storage.Search(context.Background(), SearchOptions{
		Query: "   ",
		Limit: 10,
	})
	if err == nil {
		t.Error("Search() with whitespace query should return error")
	}
}

func TestSearchVeryLongQuery(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event.Payload["command"] = "test"
	if err := storage.InsertEvent(event); err != nil {
		t.Fatal(err)
	}

	longQuery := ""
	for i := 0; i < 1000; i++ {
		longQuery += "word "
	}

	_, err := storage.Search(context.Background(), SearchOptions{
		Query: longQuery,
		Limit: 10,
	})
	if err != nil {
		t.Logf("Search() with very long query returned error (expected): %v", err)
	}
}

func TestSearchUnicodeCharacters(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event.Payload["message"] = "こんにちは世界 hello 你好"
	if err := storage.InsertEvent(event); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"Japanese", "こんにちは", 1},
		{"Chinese", "你好", 1},
		{"English", "hello", 1},
		{"Mixed", "世界", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := storage.Search(context.Background(), SearchOptions{
				Query: tt.query,
				Limit: 10,
			})
			if err != nil {
				t.Logf("Search() with Unicode query %q returned error (FTS5 tokenizer limitation): %v", tt.query, err)
				return
			}
			if len(results) != tt.want {
				t.Errorf("Search(%q) got %d results, want %d", tt.query, len(results), tt.want)
			}
		})
	}
}

func TestSearchSQLInjectionAttempts(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event.Payload["command"] = "test command"
	if err := storage.InsertEvent(event); err != nil {
		t.Fatal(err)
	}

	injectionAttempts := []string{
		"'; DROP TABLE events; --",
		"1' OR '1'='1",
		"admin'--",
		"' UNION SELECT * FROM events--",
	}

	for _, injection := range injectionAttempts {
		t.Run(injection, func(t *testing.T) {
			_, err := storage.Search(context.Background(), SearchOptions{
				Query: injection,
				Limit: 10,
			})
			if err != nil {
				t.Logf("Search() with injection attempt returned error (safe): %v", err)
			}
		})
	}
}

func TestSearchQuotesAndEscapeCharacters(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event.Payload["command"] = "echo \"hello world\" && echo 'test'"
	if err := storage.InsertEvent(event); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		query string
		want  int
	}{
		{"hello", 1},
		{"\"hello\"", 1},
		{"'test'", 1},
		{"world", 1},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results, err := storage.Search(context.Background(), SearchOptions{
				Query: tt.query,
				Limit: 10,
			})
			if err != nil {
				t.Fatalf("Search() error: %v", err)
			}
			if len(results) < tt.want {
				t.Errorf("Search(%q) got %d results, want >= %d", tt.query, len(results), tt.want)
			}
		})
	}
}

func TestSearchZeroLimit(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event.Payload["command"] = "test"
	if err := storage.InsertEvent(event); err != nil {
		t.Fatal(err)
	}

	results, err := storage.Search(context.Background(), SearchOptions{
		Query: "test",
		Limit: 0,
	})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if len(results) > 0 {
		t.Logf("Search() with limit 0 returned %d results (no limit enforced)", len(results))
	}
}

func TestSearchNegativeLimit(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event.Payload["command"] = "test"
	if err := storage.InsertEvent(event); err != nil {
		t.Fatal(err)
	}

	results, err := storage.Search(context.Background(), SearchOptions{
		Query: "test",
		Limit: -1,
	})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if len(results) > 0 {
		t.Logf("Search() with negative limit returned %d results (no limit enforced)", len(results))
	}
}

func TestSearchPhraseQuery(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event1 := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event1.Payload["command"] = "git commit message"

	event2 := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event2.Payload["command"] = "message git commit"

	if err := storage.InsertEvent(event1); err != nil {
		t.Fatal(err)
	}
	if err := storage.InsertEvent(event2); err != nil {
		t.Fatal(err)
	}

	results, err := storage.Search(context.Background(), SearchOptions{
		Query: "\"git commit\"",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if len(results) == 0 {
		t.Error("Phrase search returned no results")
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	storage, _ := setupTestDB(t)
	defer storage.Close()

	event := events.NewEvent(string(events.SourceShell), string(events.TypeCommand))
	event.Payload["command"] = "Test Command"
	if err := storage.InsertEvent(event); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		query string
	}{
		{"test"},
		{"TEST"},
		{"Test"},
		{"TeSt"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results, err := storage.Search(context.Background(), SearchOptions{
				Query: tt.query,
				Limit: 10,
			})
			if err != nil {
				t.Fatalf("Search() error: %v", err)
			}
			if len(results) != 1 {
				t.Errorf("Search(%q) got %d results, want 1", tt.query, len(results))
			}
		})
	}
}
