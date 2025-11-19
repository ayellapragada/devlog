package services

import (
	"context"
	"fmt"
	"log/slog"

	"devlog/internal/config"
	"devlog/internal/events"
	"devlog/internal/logger"
	"devlog/internal/metrics"
	"devlog/internal/storage"
)

type EventService struct {
	storage      *storage.Storage
	configGetter func() *config.Config
	logger       *logger.Logger
}

func NewEventService(storage *storage.Storage, configGetter func() *config.Config, log *logger.Logger) *EventService {
	if log == nil {
		log = logger.Default()
	}
	return &EventService{
		storage:      storage,
		configGetter: configGetter,
		logger:       log,
	}
}

func (s *EventService) IngestEvent(ctx context.Context, event *events.Event) error {
	if err := event.Validate(); err != nil {
		metrics.EventIngestionErrors.Add(1)
		return &ValidationError{Err: err}
	}

	cfg := s.configGetter()

	if event.Source == string(events.SourceShell) && event.Type == string(events.TypeCommand) {
		if command, ok := event.Payload["command"].(string); ok {
			if !cfg.ShouldCaptureCommand(command) {
				s.logger.Debug("command filtered",
					slog.String("command", command),
					slog.String("event_id", event.ID))
				return ErrEventFiltered
			}
		}
	}

	if event.Source == string(events.SourceGit) {
		if !cfg.IsModuleEnabled("git") {
			s.logger.Debug("git event filtered (module disabled)",
				slog.String("type", event.Type),
				slog.String("event_id", event.ID))
			return ErrEventFiltered
		}
	}

	insertTimer := metrics.StartTimer("insert_event")
	defer insertTimer.Stop()

	if err := s.storage.InsertEvent(event); err != nil {
		if err == storage.ErrDuplicateEvent {
			s.logger.Debug("duplicate event skipped",
				slog.String("event_id", event.ID),
				slog.String("source", event.Source))
			return ErrDuplicateEvent
		}
		metrics.EventIngestionErrors.Add(1)
		s.logger.Error("failed to store event",
			slog.String("event_id", event.ID),
			slog.String("source", event.Source),
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to store event: %w", err)
	}

	metrics.EventIngestionRate.Add(1)
	metrics.GlobalSnapshot.RecordEventIngested(event.Source, event.Type)
	s.logger.Info("event ingested",
		slog.String("source", event.Source),
		slog.String("type", event.Type),
		slog.String("event_id", event.ID))

	return nil
}

func (s *EventService) SearchEvents(ctx context.Context, opts storage.SearchOptions) ([]*storage.SearchResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 20
	}

	results, err := s.storage.Search(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return results, nil
}

func (s *EventService) GetEvents(ctx context.Context, opts storage.QueryOptions) ([]*events.Event, error) {
	return s.storage.QueryEventsContext(ctx, opts)
}

func (s *EventService) GetEventsBySource(ctx context.Context) ([]storage.SourceCount, error) {
	return s.storage.CountBySource(ctx)
}

func (s *EventService) GetTimeline(ctx context.Context) ([]storage.TimelinePoint, error) {
	return s.storage.TimelineLast7Days(ctx)
}

func (s *EventService) GetTopRepos(ctx context.Context, limit int) ([]storage.RepoStats, error) {
	return s.storage.TopRepos(ctx, limit)
}

func (s *EventService) GetTopCommands(ctx context.Context, limit int) ([]storage.CommandStats, error) {
	return s.storage.TopCommands(ctx, limit)
}

func (s *EventService) CountEvents(ctx context.Context) (int, error) {
	return s.storage.CountContext(ctx)
}

type ValidationError struct {
	Err error
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid event: %v", e.Err)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

var (
	ErrEventFiltered  = fmt.Errorf("event filtered by configuration")
	ErrDuplicateEvent = fmt.Errorf("duplicate event")
)
