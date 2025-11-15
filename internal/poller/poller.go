package poller

import (
	"time"

	"devlog/internal/events"
)

type Poller interface {
	Name() string
	PollInterval() time.Duration
	Poll() ([]*events.Event, error)
}

type Manager struct {
	pollers   []Poller
	storage   EventStorage
	stopChans map[string]chan struct{}
}

type EventStorage interface {
	InsertEvent(event *events.Event) error
}

func NewManager(storage EventStorage) *Manager {
	return &Manager{
		storage:   storage,
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
	events, err := poller.Poll()
	if err != nil {
		return
	}

	for _, event := range events {
		if err := m.storage.InsertEvent(event); err != nil {
			continue
		}
	}
}
