package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"devlog/internal/config"
	"devlog/internal/events"
	"devlog/internal/session"
	"devlog/internal/storage"
)

// Server holds the HTTP server dependencies
type Server struct {
	storage        *storage.Storage
	sessionManager *session.Manager
	config         *config.Config
	startTime      time.Time
}

// NewServer creates a new API server
func NewServer(storage *storage.Storage, sessionManager *session.Manager, cfg *config.Config) *Server {
	return &Server{
		storage:        storage,
		sessionManager: sessionManager,
		config:         cfg,
		startTime:      time.Now(),
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

	// Filter shell commands based on config
	if event.Source == events.SourceShell && event.Type == events.TypeCommand {
		if command, ok := event.Payload["command"].(string); ok {
			if !s.config.ShouldCaptureCommand(command) {
				// Silently drop filtered command - return success
				respondJSON(w, map[string]interface{}{
					"ok":       true,
					"filtered": true,
				}, http.StatusOK)
				return
			}
		}
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

// SessionHandler handles session management requests
// GET  /api/v1/sessions - List sessions
// POST /api/v1/sessions - Create a new session
func (s *Server) SessionHandler(w http.ResponseWriter, r *http.Request) {
	if s.sessionManager == nil {
		respondError(w, "Session management not enabled", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleListSessions(w, r)
	case http.MethodPost:
		s.handleCreateSession(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleListSessions handles GET requests for listing sessions
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.sessionManager.List(50, "") // Get last 50 sessions
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to list sessions: %v", err), http.StatusInternalServerError)
		return
	}

	// Format sessions for response
	sessionList := make([]map[string]interface{}, len(sessions))
	for i, sess := range sessions {
		sessionList[i] = map[string]interface{}{
			"id":          sess.ID,
			"start_time":  sess.StartTime.Format(time.RFC3339),
			"description": sess.Description,
			"event_count": len(sess.EventIDs),
			"status":      sess.Status,
		}
		if sess.EndTime != nil {
			sessionList[i]["end_time"] = sess.EndTime.Format(time.RFC3339)
			sessionList[i]["duration"] = formatDuration(sess.Duration())
		}
	}

	respondJSON(w, map[string]interface{}{
		"sessions": sessionList,
		"count":    len(sessions),
	}, http.StatusOK)
}

// handleCreateSession handles POST requests for creating sessions
func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EventIDs    []string `json:"event_ids"`
		Description string   `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if len(req.EventIDs) == 0 {
		respondError(w, "event_ids is required and must not be empty", http.StatusBadRequest)
		return
	}

	// Create session from event IDs
	sess, err := s.sessionManager.CreateFromEventIDs(req.EventIDs, req.Description, session.TriggerManual)
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to create session: %v", err), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
		"ok":          true,
		"session_id":  sess.ID,
		"description": sess.Description,
		"event_count": len(sess.EventIDs),
	}, http.StatusOK)
}

// formatDuration formats a duration in a human-readable format
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// SetupRoutes configures the HTTP routes
func (s *Server) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/ingest", s.IngestHandler)
	mux.HandleFunc("/api/v1/status", s.StatusHandler)
	mux.HandleFunc("/api/v1/sessions", s.SessionHandler)
	return mux
}
