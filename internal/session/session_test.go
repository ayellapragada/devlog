package session

import (
	"testing"
	"time"
)

func TestNewSession(t *testing.T) {
	sess := NewSession(TriggerManual)

	if sess == nil {
		t.Fatal("NewSession() returned nil")
	}

	if sess.ID == "" {
		t.Error("session ID should not be empty")
	}

	if sess.Status != StatusActive {
		t.Errorf("got status %s, want %s", sess.Status, StatusActive)
	}

	if sess.StartTrigger != TriggerManual {
		t.Errorf("got trigger %s, want %s", sess.StartTrigger, TriggerManual)
	}

	if len(sess.EventIDs) != 0 {
		t.Error("EventIDs should be empty initially")
	}

	if sess.Metadata == nil {
		t.Error("Metadata should be initialized")
	}
}

func TestNewManualSession(t *testing.T) {
	description := "Test session"
	sess := NewManualSession(description)

	if sess == nil {
		t.Fatal("NewManualSession() returned nil")
	}

	if sess.Description != description {
		t.Errorf("got description %q, want %q", sess.Description, description)
	}

	if sess.StartTrigger != TriggerManual {
		t.Errorf("got trigger %s, want %s", sess.StartTrigger, TriggerManual)
	}
}

func TestAddEvent(t *testing.T) {
	sess := NewSession(TriggerManual)

	eventIDs := []string{"event1", "event2", "event3"}
	for _, id := range eventIDs {
		sess.AddEvent(id)
	}

	if len(sess.EventIDs) != len(eventIDs) {
		t.Errorf("got %d events, want %d", len(sess.EventIDs), len(eventIDs))
	}

	for i, id := range eventIDs {
		if sess.EventIDs[i] != id {
			t.Errorf("event[%d] = %q, want %q", i, sess.EventIDs[i], id)
		}
	}
}

func TestComplete(t *testing.T) {
	sess := NewSession(TriggerManual)

	beforeComplete := time.Now()
	err := sess.Complete(TriggerPRMerge)
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}

	if sess.Status != StatusCompleted {
		t.Errorf("got status %s, want %s", sess.Status, StatusCompleted)
	}

	if sess.EndTrigger != TriggerPRMerge {
		t.Errorf("got end trigger %s, want %s", sess.EndTrigger, TriggerPRMerge)
	}

	if sess.EndTime == nil {
		t.Fatal("EndTime should not be nil")
	}

	if sess.EndTime.Before(beforeComplete) {
		t.Error("EndTime should not be before completion")
	}

	err = sess.Complete(TriggerManual)
	if err == nil {
		t.Error("Complete() should fail when session is not active")
	}
}

func TestArchive(t *testing.T) {
	sess := NewSession(TriggerManual)
	sess.Complete(TriggerManual)

	err := sess.Archive()
	if err != nil {
		t.Fatalf("Archive() error: %v", err)
	}

	if sess.Status != StatusArchived {
		t.Errorf("got status %s, want %s", sess.Status, StatusArchived)
	}

	err = sess.Archive()
	if err == nil {
		t.Error("Archive() should fail when session is already archived")
	}
}

func TestDuration(t *testing.T) {
	sess := NewSession(TriggerManual)

	time.Sleep(10 * time.Millisecond)

	activeDuration := sess.Duration()
	if activeDuration < 10*time.Millisecond {
		t.Errorf("active duration too short: %v", activeDuration)
	}

	sess.Complete(TriggerManual)

	completedDuration := sess.Duration()
	if completedDuration < 10*time.Millisecond {
		t.Errorf("completed duration too short: %v", completedDuration)
	}
}

func TestIsActive(t *testing.T) {
	sess := NewSession(TriggerManual)

	if !sess.IsActive() {
		t.Error("new session should be active")
	}

	sess.Complete(TriggerManual)

	if sess.IsActive() {
		t.Error("completed session should not be active")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		session *Session
		wantErr bool
	}{
		{
			name:    "valid session",
			session: NewSession(TriggerManual),
			wantErr: false,
		},
		{
			name: "missing ID",
			session: &Session{
				ID:           "",
				StartTime:    time.Now(),
				Status:       StatusActive,
				StartTrigger: TriggerManual,
				EventIDs:     []string{},
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			session: &Session{
				ID:           "test-id",
				StartTime:    time.Now(),
				Status:       "invalid",
				StartTrigger: TriggerManual,
				EventIDs:     []string{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.session.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSessionTriggers(t *testing.T) {
	triggers := []SessionTrigger{
		TriggerManual,
		TriggerPRMerge,
		TriggerRebase,
		TriggerAuto,
		TriggerIdle,
	}

	for _, trigger := range triggers {
		t.Run(string(trigger), func(t *testing.T) {
			sess := NewSession(trigger)
			if sess.StartTrigger != trigger {
				t.Errorf("got trigger %s, want %s", sess.StartTrigger, trigger)
			}
		})
	}
}

func TestSessionStatuses(t *testing.T) {
	sess := NewSession(TriggerManual)

	if sess.Status != StatusActive {
		t.Errorf("new session status = %s, want %s", sess.Status, StatusActive)
	}

	sess.Complete(TriggerManual)
	if sess.Status != StatusCompleted {
		t.Errorf("completed session status = %s, want %s", sess.Status, StatusCompleted)
	}

	sess.Archive()
	if sess.Status != StatusArchived {
		t.Errorf("archived session status = %s, want %s", sess.Status, StatusArchived)
	}
}
