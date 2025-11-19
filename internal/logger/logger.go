package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

const (
	DefaultLogFile = "devlog.log"
)

type Logger struct {
	*slog.Logger
	file   *os.File
	logDir string
}

func New(level slog.Level) *Logger {
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: false,
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	return &Logger{Logger: slog.New(handler)}
}

func NewWithSource(level slog.Level) *Logger {
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	return &Logger{Logger: slog.New(handler)}
}

func NewDevelopment() *Logger {
	opts := &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	return &Logger{Logger: slog.New(handler)}
}

func NewFileLogger(logDir string, level slog.Level) (*Logger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	logPath := filepath.Join(logDir, DefaultLogFile)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: false,
	}

	multiWriter := io.MultiWriter(os.Stdout, file)
	handler := slog.NewJSONHandler(multiWriter, opts)

	return &Logger{
		Logger: slog.New(handler),
		file:   file,
		logDir: logDir,
	}, nil
}

func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

func Default() *Logger {
	return New(slog.LevelInfo)
}

func DefaultFile(logDir string) (*Logger, error) {
	return NewFileLogger(logDir, slog.LevelInfo)
}
