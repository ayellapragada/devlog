package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"devlog/internal/api"
	"devlog/internal/config"
	"devlog/internal/logger"
	"devlog/internal/poller"
	"devlog/internal/queue"
	"devlog/internal/session"
	"devlog/internal/storage"
	"devlog/modules/clipboard"
	"devlog/modules/wisprflow"
)

type Daemon struct {
	config         *config.Config
	storage        *storage.Storage
	sessionManager *session.Manager
	pollerManager  *poller.Manager
	server         *http.Server
	logger         *logger.Logger
	stopChan       chan struct{}
}

func New(cfg *config.Config, store *storage.Storage) *Daemon {
	sessionManager := session.NewManager(store)
	log := logger.Default()
	pollerManager := poller.NewManager(store, log)

	return &Daemon{
		config:         cfg,
		storage:        store,
		sessionManager: sessionManager,
		pollerManager:  pollerManager,
		logger:         log,
		stopChan:       make(chan struct{}),
	}
}

func (d *Daemon) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	apiServer := api.NewServer(d.storage, d.sessionManager, d.config)
	mux := apiServer.SetupRoutes()

	addr := fmt.Sprintf("127.0.0.1:%d", d.config.HTTP.Port)
	d.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	if err := d.writePIDFile(); err != nil {
		return fmt.Errorf("write PID file: %w", err)
	}

	if err := d.processQueue(); err != nil {
		d.logger.Warn("failed to process queue", slog.String("error", err.Error()))
	}

	d.setupPollers()
	d.pollerManager.StartWithContext(ctx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		d.logger.Info("daemon started",
			slog.String("addr", addr),
			slog.Int("port", d.config.HTTP.Port),
			slog.Int("pid", os.Getpid()))
		if err := d.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case <-sigChan:
		d.logger.Info("shutdown signal received")
		cancel()
		return d.Shutdown()
	case err := <-errChan:
		d.logger.Error("server error", slog.String("error", err.Error()))
		cancel()
		d.removePIDFile()
		return fmt.Errorf("server error: %w", err)
	}
}

func (d *Daemon) processQueue() error {
	queueDir, err := config.QueueDir()
	if err != nil {
		return err
	}

	q, err := queue.New(queueDir)
	if err != nil {
		return err
	}

	queuedEvents, err := q.List()
	if err != nil {
		return err
	}

	if len(queuedEvents) == 0 {
		return nil
	}

	d.logger.Info("processing queued events", slog.Int("count", len(queuedEvents)))

	successCount := 0
	for _, event := range queuedEvents {
		if err := event.Validate(); err != nil {
			d.logger.Warn("skipping invalid queued event",
				slog.String("event_id", event.ID),
				slog.String("error", err.Error()))
			continue
		}

		if err := d.storage.InsertEvent(event); err != nil {
			d.logger.Warn("failed to insert queued event",
				slog.String("event_id", event.ID),
				slog.String("error", err.Error()))
			continue
		}

		d.logger.Debug("ingested queued event",
			slog.String("source", event.Source),
			slog.String("type", event.Type),
			slog.String("event_id", event.ID))

		if err := q.Remove(event.ID); err != nil {
			d.logger.Warn("failed to remove event from queue",
				slog.String("event_id", event.ID),
				slog.String("error", err.Error()))
		} else {
			successCount++
		}
	}

	d.logger.Info("completed queue processing",
		slog.Int("successful", successCount),
		slog.Int("total", len(queuedEvents)))
	return nil
}

func (d *Daemon) setupPollers() {
	if d.config.IsModuleEnabled("wisprflow") {
		modCfg, ok := d.config.GetModuleConfig("wisprflow")
		if !ok {
			d.logger.Warn("wisprflow module config not found")
			return
		}

		pollInterval := 60.0
		if interval, ok := modCfg["poll_interval_seconds"].(float64); ok {
			pollInterval = interval
		}

		minWords := 0
		if mw, ok := modCfg["min_words"].(float64); ok {
			minWords = int(mw)
		}

		dbPathConfig, _ := modCfg["db_path"].(string)
		homeDir, _ := os.UserHomeDir()
		dbPath := wisprflow.GetDBPath(homeDir, dbPathConfig)

		dataDir, err := config.DataDir()
		if err != nil {
			d.logger.Warn("wisprflow polling failed to get data dir",
				slog.String("error", err.Error()))
			return
		}

		wisprPoller := wisprflow.NewPoller(
			dbPath,
			dataDir,
			time.Duration(pollInterval)*time.Second,
			minWords,
		)
		d.pollerManager.Register(wisprPoller)

		d.logger.Info("wispr flow polling started",
			slog.Float64("interval_seconds", pollInterval))
	}

	if d.config.IsModuleEnabled("clipboard") {
		modCfg, ok := d.config.GetModuleConfig("clipboard")
		if !ok {
			d.logger.Warn("clipboard module config not found")
			return
		}

		pollInterval := "3s"
		if interval, ok := modCfg["poll_interval"].(string); ok {
			pollInterval = interval
		}

		maxLength := 10000
		if ml, ok := modCfg["max_length"].(float64); ok {
			maxLength = int(ml)
		}

		minLength := 1
		if ml, ok := modCfg["min_length"].(float64); ok {
			minLength = int(ml)
		}

		duration, err := time.ParseDuration(pollInterval)
		if err != nil {
			d.logger.Warn("invalid clipboard poll_interval, using default",
				slog.String("value", pollInterval),
				slog.String("error", err.Error()))
			duration = 3 * time.Second
		}

		dataDir, err := config.DataDir()
		if err != nil {
			d.logger.Warn("clipboard polling failed to get data dir",
				slog.String("error", err.Error()))
			return
		}

		clipboardPoller := clipboard.NewPoller(
			dataDir,
			duration,
			maxLength,
			minLength,
		)

		if err := clipboardPoller.Init(); err != nil {
			d.logger.Warn("clipboard poller init failed",
				slog.String("error", err.Error()))
			return
		}

		d.pollerManager.Register(clipboardPoller)

		d.logger.Info("clipboard polling started",
			slog.String("interval", pollInterval))
	}
}

func (d *Daemon) Shutdown() error {
	d.logger.Info("shutting down daemon")
	close(d.stopChan)

	if d.pollerManager != nil {
		d.pollerManager.Stop()
		d.logger.Debug("poller manager stopped")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if d.server != nil {
		if err := d.server.Shutdown(ctx); err != nil {
			d.logger.Error("failed to shutdown server", slog.String("error", err.Error()))
			return fmt.Errorf("shutdown server: %w", err)
		}
		d.logger.Debug("http server stopped")
	}

	if d.storage != nil {
		if err := d.storage.Close(); err != nil {
			d.logger.Error("failed to close storage", slog.String("error", err.Error()))
			return fmt.Errorf("close storage: %w", err)
		}
		d.logger.Debug("storage closed")
	}

	d.removePIDFile()
	d.logger.Info("daemon stopped successfully")
	return nil
}

func PIDFile() (string, error) {
	configDir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "devlogd.pid"), nil
}

func (d *Daemon) writePIDFile() error {
	pidPath, err := PIDFile()
	if err != nil {
		return err
	}

	if IsRunning() {
		return fmt.Errorf("daemon is already running (PID file exists at %s)", pidPath)
	}

	pid := os.Getpid()
	return os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644)
}

func (d *Daemon) removePIDFile() {
	pidPath, err := PIDFile()
	if err != nil {
		return
	}
	os.Remove(pidPath)
}

func IsRunning() bool {
	pidPath, err := PIDFile()
	if err != nil {
		return false
	}

	data, err := os.ReadFile(pidPath)
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func GetPID() int {
	pidPath, err := PIDFile()
	if err != nil {
		return 0
	}

	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0
	}

	return pid
}

func StopDaemon() error {
	if !IsRunning() {
		return fmt.Errorf("daemon is not running")
	}

	pid := GetPID()
	if pid == 0 {
		return fmt.Errorf("could not read PID file")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM: %w", err)
	}

	for i := 0; i < 50; i++ {
		if !IsRunning() {
			pidPath, _ := PIDFile()
			os.Remove(pidPath)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("daemon did not stop after 5 seconds")
}
