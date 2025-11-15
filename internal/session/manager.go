package session

import (
	"fmt"
	"time"
)

type Storage interface {
	InsertSession(session *Session) error
	UpdateSession(session *Session) error
	GetSession(id string) (*Session, error)
	ListSessions(limit int, status SessionStatus) ([]*Session, error)
	GetSessionEvents(sessionID string) ([]string, error)
	AddEventToSession(sessionID, eventID string) error
}

type Manager struct {
	storage Storage
}

func NewManager(storage Storage) *Manager {
	return &Manager{
		storage: storage,
	}
}

func (m *Manager) CreateFromEventIDs(eventIDs []string, description string, trigger SessionTrigger) (*Session, error) {
	if len(eventIDs) == 0 {
		return nil, fmt.Errorf("cannot create session with no events")
	}

	session := NewSession(trigger)
	session.Description = description
	session.Status = StatusActive

	if err := m.storage.InsertSession(session); err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	for _, eventID := range eventIDs {
		if err := m.storage.AddEventToSession(session.ID, eventID); err != nil {
			return nil, fmt.Errorf("add event %s to session: %w", eventID, err)
		}
	}

	return m.storage.GetSession(session.ID)
}

func (m *Manager) CreateFromTimeRange(startTime, endTime time.Time, description string, trigger SessionTrigger) (*Session, error) {
	if endTime.Before(startTime) {
		return nil, fmt.Errorf("end time must be after start time")
	}

	session := NewSession(trigger)
	session.StartTime = startTime
	session.EndTime = &endTime
	session.Description = description
	session.Status = StatusCompleted

	if err := m.storage.InsertSession(session); err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	return session, nil
}

func (m *Manager) Get(sessionID string) (*Session, error) {
	return m.storage.GetSession(sessionID)
}

func (m *Manager) List(limit int, status SessionStatus) ([]*Session, error) {
	return m.storage.ListSessions(limit, status)
}

func (m *Manager) Update(session *Session) error {
	if err := session.Validate(); err != nil {
		return fmt.Errorf("invalid session: %w", err)
	}
	return m.storage.UpdateSession(session)
}

func (m *Manager) AddEventsToSession(sessionID string, eventIDs []string) error {
	_, err := m.storage.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	for _, eventID := range eventIDs {
		if err := m.storage.AddEventToSession(sessionID, eventID); err != nil {
			return fmt.Errorf("add event %s: %w", eventID, err)
		}
	}

	return nil
}

func (m *Manager) EndSession(sessionID string, trigger SessionTrigger) error {
	session, err := m.storage.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	now := time.Now().UTC()
	session.EndTime = &now
	session.Status = StatusCompleted
	session.EndTrigger = trigger

	return m.storage.UpdateSession(session)
}
