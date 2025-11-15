package poller

import (
	"log/slog"
	"time"

	"devlog/internal/events"
	"devlog/internal/logger"
)

type Poller interface {
	Name() string
	PollInterval() time.Duration
	Poll() ([]*events.Event, error)
}

type Manager struct {
	pollers   []Poller
	storage   EventStorage
	logger    *logger.Logger
	stopChans map[string]chan struct{}
}

type EventStorage interface {
	InsertEvent(event *events.Event) error
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
	for _, p := range m.pollers {
		stopChan := make(chan struct{})
		m.stopChans[p.Name()] = stopChan
		go m.runPoller(p, stopChan)
	}
}

func (m *Manager) Stop() {
	for _, stopChan := range m.stopChans {
		close(stopChan)
	}
}

func (m *Manager) runPoller(poller Poller, stopChan chan struct{}) {
	ticker := time.NewTicker(poller.PollInterval())
	defer ticker.Stop()

	m.doPoll(poller)

	for {
		select {
		case <-ticker.C:
			m.doPoll(poller)
		case <-stopChan:
			return
		}
	}
}

func (m *Manager) doPoll(poller Poller) {
	pollerLogger := m.logger.With(slog.String("poller", poller.Name()))

	events, err := poller.Poll()
	if err != nil {
		pollerLogger.Error("poll failed", slog.String("error", err.Error()))
		return
	}

	if len(events) == 0 {
		pollerLogger.Debug("no new events")
		return
	}

	pollerLogger.Debug("poll completed", slog.Int("event_count", len(events)))

	successCount := 0
	for _, event := range events {
		if err := m.storage.InsertEvent(event); err != nil {
			pollerLogger.Error("failed to insert event",
				slog.String("event_id", event.ID),
				slog.String("error", err.Error()))
			continue
		}
		successCount++
	}

	pollerLogger.Info("events stored",
		slog.Int("successful", successCount),
		slog.Int("total", len(events)))
}
