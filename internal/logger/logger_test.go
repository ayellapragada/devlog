package logger

import (
	"log/slog"
	"testing"
)

func TestNew(t *testing.T) {
	log := New(slog.LevelInfo)
	if log == nil {
		t.Fatal("New() returned nil")
	}

	if log.Logger == nil {
		t.Error("Logger field is nil")
	}
}

func TestNewWithSource(t *testing.T) {
	log := NewWithSource(slog.LevelDebug)
	if log == nil {
		t.Fatal("NewWithSource() returned nil")
	}

	if log.Logger == nil {
		t.Error("Logger field is nil")
	}
}

func TestNewDevelopment(t *testing.T) {
	log := NewDevelopment()
	if log == nil {
		t.Fatal("NewDevelopment() returned nil")
	}

	if log.Logger == nil {
		t.Error("Logger field is nil")
	}
}

func TestDefault(t *testing.T) {
	log := Default()
	if log == nil {
		t.Fatal("Default() returned nil")
	}

	if log.Logger == nil {
		t.Error("Logger field is nil")
	}
}

func TestLogLevels(t *testing.T) {
	tests := []struct {
		name  string
		level slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := New(tt.level)
			if log == nil {
				t.Errorf("New(%v) returned nil", tt.level)
			}
		})
	}
}
