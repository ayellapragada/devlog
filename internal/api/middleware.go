package api

import (
	"log/slog"
	"net/http"
	"time"

	"devlog/internal/metrics"
)

const (
	MaxRequestSize = 1 << 20
)

func limitRequestSize(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, MaxRequestSize)
		next(w, r)
	}
}

type loggerInterface interface {
	Debug(msg string, args ...any)
}

func loggingMiddleware(logger loggerInterface, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		timer := metrics.StartAPITimer(r.URL.Path)
		defer timer.Stop()

		start := time.Now()
		next(w, r)
		duration := time.Since(start)

		logger.Debug("HTTP request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("remote_addr", r.RemoteAddr),
			slog.Duration("duration", duration))
	}
}
