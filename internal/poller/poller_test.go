package poller

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"devlog/internal/events"
	"devlog/internal/logger"
)

type mockEventService struct {
	mu             sync.Mutex
	insertedEvents []*events.Event
	shouldError    bool
}

func (m *mockEventService) IngestEvent(ctx context.Context, event *events.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldError {
		return fmt.Errorf("mock storage error")
	}

	m.insertedEvents = append(m.insertedEvents, event)
	return nil
}

func (m *mockEventService) getInsertedEvents() []*events.Event {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]*events.Event, len(m.insertedEvents))
	copy(result, m.insertedEvents)
	return result
}

type mockPoller struct {
	name           string
	interval       time.Duration
	eventsToReturn []*events.Event
	pollCount      int
	mu             sync.Mutex
}

func (m *mockPoller) Name() string {
	return m.name
}

func (m *mockPoller) PollInterval() time.Duration {
	return m.interval
}

func (m *mockPoller) Poll(ctx context.Context) ([]*events.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.pollCount++
	return m.eventsToReturn, nil
}

func (m *mockPoller) getPollCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pollCount
}

func TestNewManager(t *testing.T) {
	eventService := &mockEventService{}
	log := logger.Default()
	manager := NewManager(eventService, log)

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.eventService != eventService {
		t.Error("Manager eventService not set correctly")
	}

	if manager.stopChans == nil {
		t.Error("Manager stopChans not initialized")
	}
}

func TestManagerRegister(t *testing.T) {
	eventService := &mockEventService{}
	log := logger.Default()
	manager := NewManager(eventService, log)

	poller1 := &mockPoller{name: "test1", interval: time.Second}
	poller2 := &mockPoller{name: "test2", interval: time.Second}

	manager.Register(poller1)
	manager.Register(poller2)

	if len(manager.pollers) != 2 {
		t.Errorf("Expected 2 pollers, got %d", len(manager.pollers))
	}
}

func TestManagerStartStop(t *testing.T) {
	eventService := &mockEventService{}
	log := logger.Default()
	manager := NewManager(eventService, log)

	event1 := events.NewEvent("test", "test_type")
	event1.Payload["data"] = "test1"

	poller := &mockPoller{
		name:           "test",
		interval:       50 * time.Millisecond,
		eventsToReturn: []*events.Event{event1},
	}

	manager.Register(poller)
	manager.Start()

	time.Sleep(120 * time.Millisecond)

	manager.Stop()

	time.Sleep(50 * time.Millisecond)

	pollCount := poller.getPollCount()
	if pollCount < 2 {
		t.Errorf("Expected at least 2 polls, got %d", pollCount)
	}

	insertedEvents := eventService.getInsertedEvents()
	if len(insertedEvents) < 2 {
		t.Errorf("Expected at least 2 events inserted, got %d", len(insertedEvents))
	}
}

func TestManagerDoPoll(t *testing.T) {
	eventService := &mockEventService{}
	log := logger.Default()
	manager := NewManager(eventService, log)

	event1 := events.NewEvent("test", "type1")
	event1.Payload["id"] = "1"

	event2 := events.NewEvent("test", "type2")
	event2.Payload["id"] = "2"

	poller := &mockPoller{
		name:           "test",
		interval:       time.Second,
		eventsToReturn: []*events.Event{event1, event2},
	}

	manager.doPoll(context.Background(), poller)

	insertedEvents := eventService.getInsertedEvents()
	if len(insertedEvents) != 2 {
		t.Errorf("Expected 2 events inserted, got %d", len(insertedEvents))
	}

	if insertedEvents[0].Payload["id"] != "1" {
		t.Errorf("First event has wrong ID: %v", insertedEvents[0].Payload["id"])
	}

	if insertedEvents[1].Payload["id"] != "2" {
		t.Errorf("Second event has wrong ID: %v", insertedEvents[1].Payload["id"])
	}
}

func TestManagerDoPollWithStorageError(t *testing.T) {
	eventService := &mockEventService{shouldError: true}
	log := logger.Default()
	manager := NewManager(eventService, log)

	event := events.NewEvent("test", "type")

	poller := &mockPoller{
		name:           "test",
		interval:       time.Second,
		eventsToReturn: []*events.Event{event},
	}

	manager.doPoll(context.Background(), poller)

	insertedEvents := eventService.getInsertedEvents()
	if len(insertedEvents) != 0 {
		t.Errorf("Expected 0 events inserted on error, got %d", len(insertedEvents))
	}
}

func TestManagerDoPollWithEmptyEvents(t *testing.T) {
	eventService := &mockEventService{}
	log := logger.Default()
	manager := NewManager(eventService, log)

	poller := &mockPoller{
		name:           "test",
		interval:       time.Second,
		eventsToReturn: []*events.Event{},
	}

	manager.doPoll(context.Background(), poller)

	insertedEvents := eventService.getInsertedEvents()
	if len(insertedEvents) != 0 {
		t.Errorf("Expected 0 events inserted, got %d", len(insertedEvents))
	}
}

func TestManagerMultiplePollers(t *testing.T) {
	eventService := &mockEventService{}
	log := logger.Default()
	manager := NewManager(eventService, log)

	event1 := events.NewEvent("poller1", "type1")
	event2 := events.NewEvent("poller2", "type2")

	poller1 := &mockPoller{
		name:           "poller1",
		interval:       50 * time.Millisecond,
		eventsToReturn: []*events.Event{event1},
	}

	poller2 := &mockPoller{
		name:           "poller2",
		interval:       50 * time.Millisecond,
		eventsToReturn: []*events.Event{event2},
	}

	manager.Register(poller1)
	manager.Register(poller2)
	manager.Start()

	time.Sleep(120 * time.Millisecond)

	manager.Stop()

	insertedEvents := eventService.getInsertedEvents()
	if len(insertedEvents) < 4 {
		t.Errorf("Expected at least 4 events (2 per poller), got %d", len(insertedEvents))
	}

	hasPoller1Event := false
	hasPoller2Event := false
	for _, event := range insertedEvents {
		if event.Source == "poller1" {
			hasPoller1Event = true
		}
		if event.Source == "poller2" {
			hasPoller2Event = true
		}
	}

	if !hasPoller1Event {
		t.Error("No events from poller1 found")
	}
	if !hasPoller2Event {
		t.Error("No events from poller2 found")
	}
}
