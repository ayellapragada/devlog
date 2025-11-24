package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
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
	handler := newCompactHandler(os.Stdout, level)
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

	if fileOnly {
		handler := slog.NewJSONHandler(file, &slog.HandlerOptions{
			Level:     level,
			AddSource: false,
		})
		return &Logger{
			Logger: slog.New(handler),
			file:   file,
			logDir: logDir,
		}, nil
	}

	stdoutHandler := newCompactHandler(os.Stdout, level)
	fileHandler := slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level:     level,
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

func NewDualLogger(logDir string, stdoutLevel, fileLevel slog.Level) (*Logger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	logPath := filepath.Join(logDir, DefaultLogFile)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	stdoutHandler := newCompactHandler(os.Stdout, stdoutLevel)

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

type compactHandler struct {
	handler slog.Handler
	writer  io.Writer
	level   slog.Level
	mu      sync.Mutex
}

func newCompactHandler(w io.Writer, level slog.Level) *compactHandler {
	return &compactHandler{
		handler: slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}),
		writer:  w,
		level:   level,
	}
}

func (h *compactHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *compactHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	timeStr := r.Time.Format("15:04:05")
	level := r.Level.String()
	msg := r.Message

	fmt.Fprintf(h.writer, "%s [%s] %s", timeStr, level, msg)

	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(h.writer, " %s=%v", a.Key, a.Value)
		return true
	})

	fmt.Fprintln(h.writer)
	return nil
}

func (h *compactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &compactHandler{
		handler: h.handler.WithAttrs(attrs),
		writer:  h.writer,
		level:   h.level,
	}
}

func (h *compactHandler) WithGroup(name string) slog.Handler {
	return &compactHandler{
		handler: h.handler.WithGroup(name),
		writer:  h.writer,
		level:   h.level,
	}
}
