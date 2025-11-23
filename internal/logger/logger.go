package logger

import (
	"context"
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

func NewFileLogger(logDir string, level slog.Level, fileOnly bool) (*Logger, error) {
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

	var writer io.Writer
	if fileOnly {
		writer = file
	} else {
		writer = io.MultiWriter(os.Stdout, file)
	}
	handler := slog.NewJSONHandler(writer, opts)

	return &Logger{
		Logger: slog.New(handler),
		file:   file,
		logDir: logDir,
	}, nil
}

func NewDualLogger(logDir string, stdoutLevel, fileLevel slog.Level) (*Logger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	logPath := filepath.Join(logDir, DefaultLogFile)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	stdoutHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     stdoutLevel,
		AddSource: false,
	})

	fileHandler := slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level:     fileLevel,
		AddSource: false,
	})

	handler := &multiHandler{
		handlers: []slog.Handler{stdoutHandler, fileHandler},
	}

	return &Logger{
		Logger: slog.New(handler),
		file:   file,
		logDir: logDir,
	}, nil
}

type multiHandler struct {
	handlers []slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, record.Level) {
			if err := handler.Handle(ctx, record); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: newHandlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: newHandlers}
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
	return NewFileLogger(logDir, slog.LevelInfo, false)
}

func DefaultFileOnly(logDir string) (*Logger, error) {
	return NewFileLogger(logDir, slog.LevelDebug, true)
}

func DefaultDual(logDir string) (*Logger, error) {
	return NewDualLogger(logDir, slog.LevelInfo, slog.LevelDebug)
}
