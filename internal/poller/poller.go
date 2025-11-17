package poller

import (
	"context"
	"log/slog"
	"time"

	"devlog/internal/events"
	"devlog/internal/logger"
)

type Poller interface {
	Name() string
	PollInterval() time.Duration
	Poll(ctx context.Context) ([]*events.Event, error)
}

type Manager struct {
	pollers   []Poller
	storage   EventStorage
	logger    *logger.Logger
	stopChans map[string]chan struct{}
}

type EventStorage interface {
	InsertEvent(event *events.Event) error
	InsertEventContext(ctx context.Context, event *events.Event) error
}

func NewManager(storage EventStorage, log *logger.Logger) *Manager {
	return &Manager{
		storage:   storage,
		logger:    log,
		stopChans: make(map[string]chan struct{}),
	}
}

func (m *Manager) Register(poller Poller) {
	m.pollers = append(m.pollers, poller)
}

func (m *Manager) Start() {
	m.StartWithContext(context.Background())
}

func (m *Manager) StartWithContext(ctx context.Context) {
	for _, p := range m.pollers {
		stopChan := make(chan struct{})
		m.stopChans[p.Name()] = stopChan
		go m.runPoller(ctx, p, stopChan)
	}
}

func (m *Manager) Stop() {
	for _, stopChan := range m.stopChans {
		close(stopChan)
	}
}

func (m *Manager) runPoller(ctx context.Context, poller Poller, stopChan chan struct{}) {
	ticker := time.NewTicker(poller.PollInterval())
	defer ticker.Stop()

	m.doPoll(ctx, poller)

	for {
		select {
		case <-ticker.C:
			m.doPoll(ctx, poller)
		case <-stopChan:
			m.logger.Debug("poller stopped", slog.String("poller", poller.Name()))
			return
		case <-ctx.Done():
			m.logger.Info("poller cancelled",
				slog.String("poller", poller.Name()),
				slog.String("reason", ctx.Err().Error()))
			return
		}
	}
}

func (m *Manager) doPoll(ctx context.Context, poller Poller) {
	pollerLogger := m.logger.With(slog.String("poller", poller.Name()))

	pollCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	events, err := poller.Poll(pollCtx)
	if err != nil {
		if ctx.Err() != nil {
			pollerLogger.Debug("poll cancelled", slog.String("error", ctx.Err().Error()))
		} else {
			pollerLogger.Error("poll failed", slog.String("error", err.Error()))
		}
		return
	}

	if len(events) == 0 {
		pollerLogger.Debug("no new events")
		return
	}

	pollerLogger.Debug("poll completed", slog.Int("event_count", len(events)))

	successCount := 0
	for _, event := range events {
		insertCtx, insertCancel := context.WithTimeout(ctx, 5*time.Second)
		err := m.storage.InsertEventContext(insertCtx, event)
		insertCancel()

		if err != nil {
			pollerLogger.Debug("skipping event",
				slog.String("event_id", event.ID),
				slog.String("reason", err.Error()))
			continue
		}
		successCount++
	}

	pollerLogger.Info("events stored",
		slog.Int("successful", successCount),
		slog.Int("total", len(events)))
}
