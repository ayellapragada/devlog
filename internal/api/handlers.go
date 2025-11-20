package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"devlog/internal/config"
	"devlog/internal/events"
	"devlog/internal/logger"
	"devlog/internal/metrics"
	"devlog/internal/services"
	"devlog/internal/storage"
)

const (
	DefaultEventsLimit      = 50
	DefaultSearchLimit      = 20
	MaxSearchLimit          = 100
	DefaultTopReposLimit    = 10
	DefaultTopCommandsLimit = 15
	HealthCheckTimeout      = 2 * time.Second
	MaxQueryLength          = 1000
)

type Server struct {
	storage      *storage.Storage
	eventService *services.EventService
	config       *config.Config
	logger       *logger.Logger
	startTime    time.Time
}

func NewServer(storage *storage.Storage, configGetter func() *config.Config) *Server {
	log := logger.Default()
	eventService := services.NewEventService(storage, configGetter, log)
	cfg := configGetter()
	return &Server{
		storage:      storage,
		eventService: eventService,
		config:       cfg,
		logger:       log,
		startTime:    time.Now(),
	}
}

func (s *Server) IngestHandler(w http.ResponseWriter, r *http.Request) {
	timer := metrics.StartAPITimer("/api/v1/ingest")
	defer timer.Stop()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		metrics.EventIngestionErrors.Add(1)
		respondJSON(w, ErrorResponse{
			OK:    false,
			Error: "Failed to read request body",
		}, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	event, err := events.FromJSON(body)
	if err != nil {
		metrics.EventIngestionErrors.Add(1)
		respondJSON(w, ErrorResponse{
			OK:    false,
			Error: fmt.Sprintf("Invalid event JSON: %v", err),
		}, http.StatusBadRequest)
		return
	}

	err = s.eventService.IngestEvent(r.Context(), event)
	if err == services.ErrEventFiltered {
		respondJSON(w, IngestEventResponse{
			OK:       true,
			Filtered: true,
		}, http.StatusOK)
		return
	}
	if err != nil {
		var validationErr *services.ValidationError
		if errors.Is(err, validationErr) || errors.As(err, &validationErr) {
			respondJSON(w, ErrorResponse{
				OK:    false,
				Error: err.Error(),
			}, http.StatusBadRequest)
			return
		}
		respondJSON(w, ErrorResponse{
			OK:    false,
			Error: err.Error(),
		}, http.StatusInternalServerError)
		return
	}

	respondJSON(w, IngestEventResponse{
		OK:      true,
		EventID: event.ID,
	}, http.StatusOK)
}

func (s *Server) StatusHandler(w http.ResponseWriter, r *http.Request) {
	count, err := s.storage.Count()
	if err != nil {
		respondJSON(w, ErrorResponse{
			OK:    false,
			Error: fmt.Sprintf("Failed to count events: %v", err),
		}, http.StatusInternalServerError)
		return
	}

	uptime := time.Since(s.startTime).Seconds()

	respondJSON(w, StatusResponse{
		Running:       true,
		EventCount:    count,
		UptimeSeconds: int(uptime),
	}, http.StatusOK)
}

func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]string)
	allHealthy := true

	ctx, cancel := context.WithTimeout(r.Context(), HealthCheckTimeout)
	defer cancel()

	_, err := s.eventService.CountEvents(ctx)
	if err != nil {
		checks["database"] = "unhealthy: " + err.Error()
		allHealthy = false
	} else {
		checks["database"] = "healthy"
	}

	status := "healthy"
	statusCode := http.StatusOK
	if !allHealthy {
		status = "unhealthy"
		statusCode = http.StatusServiceUnavailable
	}

	respondJSON(w, HealthResponse{
		Status: status,
		Checks: checks,
	}, statusCode)
}

func respondJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, message string, statusCode int) {
	respondJSON(w, ErrorResponse{
		OK:    false,
		Error: message,
	}, statusCode)
}

func (s *Server) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	limit := DefaultEventsLimit
	events, err := s.eventService.GetEvents(r.Context(), storage.QueryOptions{
		Limit: limit,
	})
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to query events: %v", err), http.StatusInternalServerError)
		return
	}

	eventList := make([]EventResponse, len(events))
	for i, evt := range events {
		eventList[i] = EventResponse{
			ID:        evt.ID,
			Timestamp: evt.Timestamp,
			Source:    evt.Source,
			Type:      evt.Type,
			Repo:      evt.Repo,
			Branch:    evt.Branch,
			Payload:   evt.Payload,
		}
	}

	respondJSON(w, GetEventsResponse{
		Events: eventList,
		Count:  len(events),
	}, http.StatusOK)
}

func (s *Server) handleEventsBySource(w http.ResponseWriter, r *http.Request) {
	results, err := s.eventService.GetEventsBySource(r.Context())
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to query events: %v", err), http.StatusInternalServerError)
		return
	}

	data := make([]SourceCount, len(results))
	for i, sc := range results {
		data[i] = SourceCount{
			Source: sc.Source,
			Count:  sc.Count,
		}
	}

	respondJSON(w, EventsBySourceResponse{Data: data}, http.StatusOK)
}

func (s *Server) handleEventsTimeline(w http.ResponseWriter, r *http.Request) {
	results, err := s.eventService.GetTimeline(r.Context())
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to query timeline: %v", err), http.StatusInternalServerError)
		return
	}

	data := make([]TimelinePoint, len(results))
	for i, tp := range results {
		data[i] = TimelinePoint{
			Hour:  tp.Hour,
			Count: tp.Count,
		}
	}

	respondJSON(w, EventsTimelineResponse{Data: data}, http.StatusOK)
}

func (s *Server) handleRepoStats(w http.ResponseWriter, r *http.Request) {
	results, err := s.eventService.GetTopRepos(r.Context(), DefaultTopReposLimit)
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to query repos: %v", err), http.StatusInternalServerError)
		return
	}

	data := make([]RepoStat, len(results))
	for i, rs := range results {
		data[i] = RepoStat{
			Repo:  rs.Repo,
			Count: rs.Count,
		}
	}

	respondJSON(w, RepoStatsResponse{Data: data}, http.StatusOK)
}

func (s *Server) handleCommandStats(w http.ResponseWriter, r *http.Request) {
	results, err := s.eventService.GetTopCommands(r.Context(), DefaultTopCommandsLimit)
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to query commands: %v", err), http.StatusInternalServerError)
		return
	}

	data := make([]CommandStat, len(results))
	for i, cs := range results {
		data[i] = CommandStat{
			Command: cs.Command,
			Count:   cs.Count,
		}
	}

	respondJSON(w, CommandStatsResponse{Data: data}, http.StatusOK)
}

func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		if days == "" {
			return 0, fmt.Errorf("invalid duration: missing number before 'd'")
		}
		var d int
		_, err := fmt.Sscanf(days, "%d", &d)
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %w", err)
		}
		if d < 0 {
			return 0, fmt.Errorf("invalid duration: negative value not allowed")
		}
		return time.Duration(d) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		query = "*"
	}

	if utf8.RuneCountInString(query) > MaxQueryLength {
		respondError(w, fmt.Sprintf("query parameter 'q' exceeds maximum length of %d characters", MaxQueryLength), http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := DefaultSearchLimit
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			respondError(w, fmt.Sprintf("invalid limit format: %v", err), http.StatusBadRequest)
			return
		}

		if limit <= 0 {
			limit = DefaultSearchLimit
		} else if limit > MaxSearchLimit {
			respondError(w, fmt.Sprintf("limit exceeds maximum of %d", MaxSearchLimit), http.StatusBadRequest)
			return
		}
	}

	cursor := r.URL.Query().Get("cursor")
	if cursor != "" {
		decoded, err := base64.URLEncoding.DecodeString(cursor)
		if err != nil {
			respondError(w, fmt.Sprintf("invalid cursor format: %v", err), http.StatusBadRequest)
			return
		}

		_, err = strconv.Atoi(string(decoded))
		if err != nil {
			respondError(w, fmt.Sprintf("invalid cursor format: %v", err), http.StatusBadRequest)
			return
		}
	}

	searchOpts := storage.SearchOptions{
		Query:         query,
		Limit:         limit,
		Cursor:        cursor,
		Modules:       r.URL.Query()["module"],
		Types:         r.URL.Query()["type"],
		RepoPattern:   r.URL.Query().Get("repo"),
		BranchPattern: r.URL.Query().Get("branch"),
	}

	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		duration, err := parseDuration(sinceStr)
		if err != nil {
			respondError(w, fmt.Sprintf("invalid since duration: %v", err), http.StatusBadRequest)
			return
		}
		afterTime := time.Now().Add(-duration)
		searchOpts.After = &afterTime
	}

	sortOrder := r.URL.Query().Get("sort")
	if sortOrder == "" {
		sortOrder = "relevance"
	}
	switch sortOrder {
	case "relevance":
		searchOpts.SortOrder = storage.SortByRelevance
	case "time_desc":
		searchOpts.SortOrder = storage.SortByTimeDesc
	default:
		respondError(w, fmt.Sprintf("invalid sort order: %s (must be relevance or time_desc)", sortOrder), http.StatusBadRequest)
		return
	}

	results, err := s.eventService.SearchEvents(r.Context(), searchOpts)
	if err != nil {
		respondError(w, fmt.Sprintf("Search failed: %v", err), http.StatusInternalServerError)
		return
	}

	data := make([]SearchResultResponse, len(results))
	var nextCursor string
	for i, result := range results {
		data[i] = SearchResultResponse{
			ID:        result.Event.ID,
			Timestamp: result.Event.Timestamp,
			Source:    result.Event.Source,
			Type:      result.Event.Type,
			Repo:      result.Event.Repo,
			Branch:    result.Event.Branch,
			Payload:   result.Event.Payload,
			Rank:      result.Rank,
		}
		if result.NextCursor != "" {
			nextCursor = result.NextCursor
		}
	}

	response := SearchResponse{
		Results: data,
		Count:   len(data),
		Query:   query,
	}
	if nextCursor != "" {
		response.NextCursor = nextCursor
		response.HasMore = true
	}

	respondJSON(w, response, http.StatusOK)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	summary := r.URL.Query().Get("summary") == "true"

	if summary {
		summaryData := metrics.GlobalSnapshot.GetSummary()
		respondJSON(w, summaryData, http.StatusOK)
	} else {
		jsonData, err := metrics.GlobalSnapshot.ToJSON()
		if err != nil {
			respondError(w, fmt.Sprintf("Failed to generate metrics: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonData)
	}
}

func (s *Server) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	ingestHandler := loggingMiddleware(s.logger, limitRequestSize(s.IngestHandler))
	statusHandler := loggingMiddleware(s.logger, s.StatusHandler)
	healthHandler := loggingMiddleware(s.logger, s.HealthHandler)

	eventsHandler := loggingMiddleware(s.logger, s.handleGetEvents)
	eventsBySourceHandler := loggingMiddleware(s.logger, s.handleEventsBySource)
	eventsTimelineHandler := loggingMiddleware(s.logger, s.handleEventsTimeline)
	repoStatsHandler := loggingMiddleware(s.logger, s.handleRepoStats)
	commandStatsHandler := loggingMiddleware(s.logger, s.handleCommandStats)

	mux.HandleFunc("POST /api/v1/ingest", ingestHandler)
	mux.HandleFunc("GET /api/v1/status", statusHandler)
	mux.HandleFunc("GET /api/v1/health", healthHandler)

	mux.HandleFunc("GET /api/v1/events", eventsHandler)
	mux.HandleFunc("GET /api/v1/search", loggingMiddleware(s.logger, s.handleSearch))
	mux.HandleFunc("GET /api/v1/metrics", loggingMiddleware(s.logger, s.handleMetrics))
	mux.HandleFunc("GET /api/v1/analytics/events-by-source", eventsBySourceHandler)
	mux.HandleFunc("GET /api/v1/analytics/events-timeline", eventsTimelineHandler)
	mux.HandleFunc("GET /api/v1/analytics/repo-stats", repoStatsHandler)
	mux.HandleFunc("GET /api/v1/analytics/command-stats", commandStatsHandler)

	mux.HandleFunc("GET /", s.handleFrontend)

	return mux
}
