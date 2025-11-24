package poller

import (
	"context"
	"log/slog"
	"sync"
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
	pollers      map[string]Poller
	eventService EventService
	logger       *logger.Logger
	stopChans    map[string]chan struct{}
	stopOnce     map[string]*sync.Once
	running      map[string]bool
	mu           sync.RWMutex
}

type EventService interface {
	IngestEvent(ctx context.Context, event *events.Event) error
}

func NewManager(eventService EventService, log *logger.Logger) *Manager {
	return &Manager{
		pollers:      make(map[string]Poller),
		eventService: eventService,
		logger:       log,
		stopChans:    make(map[string]chan struct{}),
		stopOnce:     make(map[string]*sync.Once),
		running:      make(map[string]bool),
	}
}

func (m *Manager) Register(poller Poller) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pollers[poller.Name()] = poller
}

func (m *Manager) Start() {
	m.StartWithContext(context.Background())
}

func (m *Manager) StartWithContext(ctx context.Context) {
	m.mu.RLock()
	pollers := make([]Poller, 0, len(m.pollers))
	for _, p := range m.pollers {
		pollers = append(pollers, p)
	}
	m.mu.RUnlock()

	for _, p := range pollers {
		m.StartPoller(ctx, p)
	}
}

func (m *Manager) StartPoller(ctx context.Context, poller Poller) {
	if ctx == nil {
		ctx = context.Background()
	}

	name := poller.Name()

	m.mu.Lock()
	// Check if poller is already running and stop it first
	if m.running[name] {
		stopChan := m.stopChans[name]
		stopOnce := m.stopOnce[name]
		m.mu.Unlock()

		// Stop existing poller
		if stopChan != nil && stopOnce != nil {
			stopOnce.Do(func() {
				close(stopChan)
			})
		}

		// Wait a brief moment for the goroutine to exit
		time.Sleep(10 * time.Millisecond)

		m.mu.Lock()
	}

	// Create new stop channel and once
	stopChan := make(chan struct{})
	m.stopChans[name] = stopChan
	m.stopOnce[name] = &sync.Once{}
	m.running[name] = true
	m.mu.Unlock()

	go m.runPoller(ctx, poller, stopChan, name)
}

func (m *Manager) StopPoller(name string) {
	m.mu.Lock()
	stopChan, exists := m.stopChans[name]
	stopOnce, onceExists := m.stopOnce[name]
	if !exists || !onceExists {
		m.mu.Unlock()
		return
	}
	delete(m.stopChans, name)
	delete(m.stopOnce, name)
	m.running[name] = false
	m.mu.Unlock()

	// Use sync.Once to make channel closing idempotent
	stopOnce.Do(func() {
		close(stopChan)
	})
}

func (m *Manager) RestartPoller(ctx context.Context, name string) {
	if ctx == nil {
		ctx = context.Background()
	}

	m.StopPoller(name)

	// Wait a brief moment for the goroutine to exit
	time.Sleep(10 * time.Millisecond)

	m.mu.RLock()
	poller, exists := m.pollers[name]
	m.mu.RUnlock()

	if exists {
		m.StartPoller(ctx, poller)
	}
}

func (m *Manager) Stop() {
	m.mu.RLock()
	names := make([]string, 0, len(m.stopChans))
	for name := range m.stopChans {
		names = append(names, name)
	}
	m.mu.RUnlock()

	for _, name := range names {
		m.StopPoller(name)
	}
}

func (m *Manager) runPoller(ctx context.Context, poller Poller, stopChan chan struct{}, name string) {
	defer func() {
		m.mu.Lock()
		m.running[name] = false
		m.mu.Unlock()
	}()

	ticker := time.NewTicker(poller.PollInterval())
	defer ticker.Stop()

	m.doPoll(ctx, poller)

	for {
		select {
		case <-ticker.C:
			m.doPoll(ctx, poller)
		case <-stopChan:
			m.logger.Debug("poller stopped", slog.String("poller", name))
			return
		case <-ctx.Done():
			m.logger.Info("poller cancelled",
				slog.String("poller", name),
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
		return
	}

	pollerLogger.Debug("poll completed", slog.Int("event_count", len(events)))

	successCount := 0
	for _, event := range events {
		insertCtx, insertCancel := context.WithTimeout(ctx, 5*time.Second)
		err := m.eventService.IngestEvent(insertCtx, event)
		insertCancel()

		if err != nil {
			pollerLogger.Debug("skipping event",
				slog.String("event_id", event.ID),
				slog.String("reason", err.Error()))
			continue
		}
		successCount++
	}

	pollerLogger.Debug("events stored",
		slog.Int("successful", successCount),
		slog.Int("total", len(events)))
}
