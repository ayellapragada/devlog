package session

import (
	"fmt"
	"time"
)

// Storage interface defines the required storage operations for sessions
type Storage interface {
	InsertSession(session *Session) error
	UpdateSession(session *Session) error
	GetSession(id string) (*Session, error)
	ListSessions(limit int, status SessionStatus) ([]*Session, error)
	GetSessionEvents(sessionID string) ([]string, error)
	AddEventToSession(sessionID, eventID string) error
}

// Manager handles session creation and management
type Manager struct {
	storage Storage
}

// NewManager creates a new session manager
func NewManager(storage Storage) *Manager {
	return &Manager{
		storage: storage,
	}
}

// CreateFromEventIDs creates a new session from a list of event IDs
func (m *Manager) CreateFromEventIDs(eventIDs []string, description string, trigger SessionTrigger) (*Session, error) {
	if len(eventIDs) == 0 {
		return nil, fmt.Errorf("cannot create session with no events")
	}

	// Create session
	session := NewSession(trigger)
	session.Description = description
	session.Status = StatusActive // Start as active

	// Store session
	if err := m.storage.InsertSession(session); err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	// Add all events to the session
	for _, eventID := range eventIDs {
		if err := m.storage.AddEventToSession(session.ID, eventID); err != nil {
			return nil, fmt.Errorf("add event %s to session: %w", eventID, err)
		}
	}

	// Reload session with events to get full data
	return m.storage.GetSession(session.ID)
}

// CreateFromTimeRange creates a new session from events in a time range
func (m *Manager) CreateFromTimeRange(startTime, endTime time.Time, description string, trigger SessionTrigger) (*Session, error) {
	if endTime.Before(startTime) {
		return nil, fmt.Errorf("end time must be after start time")
	}

	// Create session
	session := NewSession(trigger)
	session.StartTime = startTime
	session.EndTime = &endTime
	session.Description = description
	session.Status = StatusCompleted

	// Store session
	if err := m.storage.InsertSession(session); err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	// Note: Events will be associated with this session separately
	// The caller will need to query events in this time range and add them

	return session, nil
}

// Get retrieves a session by ID
func (m *Manager) Get(sessionID string) (*Session, error) {
	return m.storage.GetSession(sessionID)
}

// List retrieves sessions with optional filters
func (m *Manager) List(limit int, status SessionStatus) ([]*Session, error) {
	return m.storage.ListSessions(limit, status)
}

// Update updates an existing session
func (m *Manager) Update(session *Session) error {
	if err := session.Validate(); err != nil {
		return fmt.Errorf("invalid session: %w", err)
	}
	return m.storage.UpdateSession(session)
}

// AddEventsToSession adds events to an existing session
func (m *Manager) AddEventsToSession(sessionID string, eventIDs []string) error {
	// Verify session exists
	_, err := m.storage.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	// Add events
	for _, eventID := range eventIDs {
		if err := m.storage.AddEventToSession(sessionID, eventID); err != nil {
			return fmt.Errorf("add event %s: %w", eventID, err)
		}
	}

	return nil
}

// EndSession marks a session as completed
func (m *Manager) EndSession(sessionID string, trigger SessionTrigger) error {
	// Get session
	session, err := m.storage.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	// Update session
	now := time.Now().UTC()
	session.EndTime = &now
	session.Status = StatusCompleted
	session.EndTrigger = trigger

	return m.storage.UpdateSession(session)
}
