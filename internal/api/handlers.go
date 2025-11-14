package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"devlog/internal/events"
	"devlog/internal/storage"
)

// Server holds the HTTP server dependencies
type Server struct {
	storage   *storage.Storage
	startTime time.Time
}

// NewServer creates a new API server
func NewServer(storage *storage.Storage) *Server {
	return &Server{
		storage:   storage,
		startTime: time.Now(),
	}
}

// IngestHandler handles POST /api/v1/ingest
func (s *Server) IngestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	event, err := events.FromJSON(body)
	if err != nil {
		respondError(w, fmt.Sprintf("Invalid event JSON: %v", err), http.StatusBadRequest)
		return
	}

	if err := event.Validate(); err != nil {
		respondError(w, fmt.Sprintf("Invalid event: %v", err), http.StatusBadRequest)
		return
	}

	if err := s.storage.InsertEvent(event); err != nil {
		respondError(w, fmt.Sprintf("Failed to store event: %v", err), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
		"ok":       true,
		"event_id": event.ID,
	}, http.StatusOK)
}

// StatusHandler handles GET /api/v1/status
func (s *Server) StatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	count, err := s.storage.Count()
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to count events: %v", err), http.StatusInternalServerError)
		return
	}

	uptime := time.Since(s.startTime).Seconds()

	respondJSON(w, map[string]interface{}{
		"running":        true,
		"event_count":    count,
		"uptime_seconds": int(uptime),
	}, http.StatusOK)
}

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// respondError writes an error JSON response
func respondError(w http.ResponseWriter, message string, statusCode int) {
	respondJSON(w, map[string]interface{}{
		"ok":    false,
		"error": message,
	}, statusCode)
}

// SetupRoutes configures the HTTP routes
func (s *Server) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/ingest", s.IngestHandler)
	mux.HandleFunc("/api/v1/status", s.StatusHandler)
	return mux
}
