package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"devlog/internal/config"
	"devlog/internal/events"
	"devlog/internal/session"
	"devlog/internal/storage"
)

func setupTestServer(t *testing.T) (*Server, *storage.Storage) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	if err := storage.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB() error: %v", err)
	}

	store, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	sessionManager := session.NewManager(store)
	cfg := config.DefaultConfig()
	server := NewServer(store, sessionManager, cfg)
	return server, store
}

func TestIngestHandler(t *testing.T) {
	server, store := setupTestServer(t)
	defer store.Close()

	event := events.NewEvent(events.SourceGit, events.TypeCommit)
	event.Repo = "/path/to/repo"
	event.Branch = "main"
	event.Payload["hash"] = "abc123"

	eventJSON, err := event.ToJSON()
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()

	server.IngestHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response["ok"] != true {
		t.Errorf("got ok=%v, want true", response["ok"])
	}

	if response["event_id"] != event.ID {
		t.Errorf("got event_id=%v, want %s", response["event_id"], event.ID)
	}

	// Verify event was stored
	count, err := store.Count()
	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Errorf("got count %d, want 1", count)
	}
}

func TestIngestHandlerInvalidMethod(t *testing.T) {
	server, store := setupTestServer(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ingest", nil)
	w := httptest.NewRecorder()

	server.IngestHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("got status %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestIngestHandlerInvalidJSON(t *testing.T) {
	server, store := setupTestServer(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.IngestHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response["ok"] != false {
		t.Errorf("got ok=%v, want false", response["ok"])
	}
}

func TestIngestHandlerInvalidEvent(t *testing.T) {
	server, store := setupTestServer(t)
	defer store.Close()

	// Event with invalid source
	invalidEvent := map[string]interface{}{
		"v":         1,
		"id":        "123e4567-e89b-12d3-a456-426614174000",
		"timestamp": "2025-11-14T10:00:00Z",
		"source":    "invalid_source",
		"type":      "commit",
		"payload":   map[string]interface{}{},
	}

	eventJSON, _ := json.Marshal(invalidEvent)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader(eventJSON))
	w := httptest.NewRecorder()

	server.IngestHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestStatusHandler(t *testing.T) {
	server, store := setupTestServer(t)
	defer store.Close()

	// Insert some events first
	for i := 0; i < 5; i++ {
		event := events.NewEvent(events.SourceGit, events.TypeCommit)
		if err := store.InsertEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	w := httptest.NewRecorder()

	server.StatusHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response["running"] != true {
		t.Errorf("got running=%v, want true", response["running"])
	}

	// event_count comes back as float64 from JSON
	eventCount := int(response["event_count"].(float64))
	if eventCount != 5 {
		t.Errorf("got event_count=%d, want 5", eventCount)
	}

	if _, ok := response["uptime_seconds"]; !ok {
		t.Error("missing uptime_seconds field")
	}
}

func TestStatusHandlerInvalidMethod(t *testing.T) {
	server, store := setupTestServer(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/status", nil)
	w := httptest.NewRecorder()

	server.StatusHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("got status %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestSetupRoutes(t *testing.T) {
	server, store := setupTestServer(t)
	defer store.Close()

	mux := server.SetupRoutes()
	if mux == nil {
		t.Fatal("SetupRoutes() returned nil")
	}

	// Test that routes are registered by making requests
	event := events.NewEvent(events.SourceGit, events.TypeCommit)
	eventJSON, _ := event.ToJSON()

	ingestReq := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader(eventJSON))
	ingestW := httptest.NewRecorder()
	mux.ServeHTTP(ingestW, ingestReq)

	if ingestW.Code != http.StatusOK {
		t.Errorf("ingest route: got status %d, want %d", ingestW.Code, http.StatusOK)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	statusW := httptest.NewRecorder()
	mux.ServeHTTP(statusW, statusReq)

	if statusW.Code != http.StatusOK {
		t.Errorf("status route: got status %d, want %d", statusW.Code, http.StatusOK)
	}
}

func TestSessionHandlerCreate(t *testing.T) {
	server, store := setupTestServer(t)
	defer store.Close()

	// Create some events first
	event1 := events.NewEvent(events.SourceGit, events.TypeCommit)
	event1.Repo = "/path/to/repo"
	store.InsertEvent(event1)

	event2 := events.NewEvent(events.SourceGit, events.TypeCommit)
	event2.Repo = "/path/to/repo"
	store.InsertEvent(event2)

	// Create session request
	reqBody := map[string]interface{}{
		"event_ids":   []string{event1.ID, event2.ID},
		"description": "Test session",
	}

	reqJSON, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewReader(reqJSON))
	w := httptest.NewRecorder()

	server.SessionHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response["ok"] != true {
		t.Errorf("got ok=%v, want true", response["ok"])
	}

	sessionID, ok := response["session_id"].(string)
	if !ok || sessionID == "" {
		t.Error("session_id is missing or empty")
	}

	if response["description"] != "Test session" {
		t.Errorf("got description=%v, want 'Test session'", response["description"])
	}

	eventCount := int(response["event_count"].(float64))
	if eventCount != 2 {
		t.Errorf("got event_count=%d, want 2", eventCount)
	}
}

func TestSessionHandlerCreateEmptyEventIDs(t *testing.T) {
	server, store := setupTestServer(t)
	defer store.Close()

	reqBody := map[string]interface{}{
		"event_ids":   []string{},
		"description": "Empty session",
	}

	reqJSON, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewReader(reqJSON))
	w := httptest.NewRecorder()

	server.SessionHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if response["ok"] != false {
		t.Errorf("got ok=%v, want false", response["ok"])
	}
}

func TestSessionHandlerCreateInvalidJSON(t *testing.T) {
	server, store := setupTestServer(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.SessionHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSessionHandlerList(t *testing.T) {
	server, store := setupTestServer(t)
	defer store.Close()

	// Create some events and sessions
	event1 := events.NewEvent(events.SourceGit, events.TypeCommit)
	store.InsertEvent(event1)

	event2 := events.NewEvent(events.SourceGit, events.TypeCommit)
	store.InsertEvent(event2)

	// Create two sessions
	reqBody1 := map[string]interface{}{
		"event_ids":   []string{event1.ID},
		"description": "Session 1",
	}
	reqJSON1, _ := json.Marshal(reqBody1)
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewReader(reqJSON1))
	w1 := httptest.NewRecorder()
	server.SessionHandler(w1, req1)

	reqBody2 := map[string]interface{}{
		"event_ids":   []string{event2.ID},
		"description": "Session 2",
	}
	reqJSON2, _ := json.Marshal(reqBody2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewReader(reqJSON2))
	w2 := httptest.NewRecorder()
	server.SessionHandler(w2, req2)

	// Now list sessions
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
	w := httptest.NewRecorder()

	server.SessionHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	sessions, ok := response["sessions"].([]interface{})
	if !ok {
		t.Fatal("sessions field is missing or not an array")
	}

	if len(sessions) != 2 {
		t.Errorf("got %d sessions, want 2", len(sessions))
	}

	count := int(response["count"].(float64))
	if count != 2 {
		t.Errorf("got count=%d, want 2", count)
	}
}

func TestSessionHandlerListEmpty(t *testing.T) {
	server, store := setupTestServer(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
	w := httptest.NewRecorder()

	server.SessionHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	sessions, ok := response["sessions"].([]interface{})
	if !ok {
		t.Fatal("sessions field is missing or not an array")
	}

	if len(sessions) != 0 {
		t.Errorf("got %d sessions, want 0", len(sessions))
	}
}

func TestSessionHandlerInvalidMethod(t *testing.T) {
	server, store := setupTestServer(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions", nil)
	w := httptest.NewRecorder()

	server.SessionHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("got status %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}
