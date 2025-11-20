package daemon

import (
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"devlog/internal/api"
	"devlog/internal/config"
	"devlog/internal/errors"
	"devlog/internal/logger"
	"devlog/internal/metrics"
	"devlog/internal/poller"
	"devlog/internal/queue"
	"devlog/internal/services"
	"devlog/internal/storage"
	_ "devlog/modules/claude"
	_ "devlog/modules/clipboard"
	_ "devlog/modules/wisprflow"
	_ "devlog/plugins/summarizer"
)

const (
	PluginShutdownTimeout      = 5 * time.Second
	PluginShutdownTimeoutShort = 2 * time.Second
	ServerShutdownTimeout      = 10 * time.Second
	ServerShutdownTimeoutShort = 2 * time.Second
	StopDaemonPollInterval     = 100 * time.Millisecond
	StopDaemonMaxAttempts      = 50
	QueueProcessorInterval     = 30 * time.Second
	MetricsUpdaterInterval     = 60 * time.Second
)

type Daemon struct {
	config          *config.Config
	configMu        sync.RWMutex
	configWatcher   *config.Watcher
	storage         *storage.Storage
	eventService    poller.EventService
	pollerManager   *poller.Manager
	server          *http.Server
	logger          *logger.Logger
	stopChan        chan struct{}
	pluginCtx       context.Context
	pluginCtxCancel context.CancelFunc
	pluginWG        sync.WaitGroup
	plugins         map[string]*pluginInstance
	pluginsMu       sync.RWMutex
	modules         map[string]string
	modulesMu       sync.RWMutex
	moduleCtx       context.Context
}

func New(cfg *config.Config, store *storage.Storage) *Daemon {
	logDir, err := config.DataDir()
	var log *logger.Logger
	if err == nil {
		fileLog, err := logger.DefaultFile(logDir)
		if err == nil {
			log = fileLog
		} else {
			log = logger.Default()
		}
	} else {
		log = logger.Default()
	}

	d := &Daemon{
		config:   cfg,
		storage:  store,
		logger:   log,
		stopChan: make(chan struct{}),
		plugins:  make(map[string]*pluginInstance),
		modules:  make(map[string]string),
	}

	eventService := services.NewEventService(store, d.getConfig, log)
	d.eventService = eventService
	d.pollerManager = poller.NewManager(eventService, log)

	return d
}

func (d *Daemon) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	var startupComplete bool

	defer func() {
		if !startupComplete {
			cancel()
			d.cleanupOnError()
		}
	}()

	if err := d.preStartupValidation(); err != nil {
		return errors.WrapDaemon("pre-startup validation", err)
	}

	if err := d.setupResources(ctx); err != nil {
		return errors.WrapDaemon("setup resources", err)
	}

	if err := d.startServices(ctx); err != nil {
		return errors.WrapDaemon("start services", err)
	}

	startupComplete = true
	d.logger.Info("daemon started successfully",
		slog.Int("port", d.config.HTTP.Port),
		slog.Int("pid", os.Getpid()))

	return d.runEventLoop(ctx, cancel)
}

func (d *Daemon) getConfig() *config.Config {
	d.configMu.RLock()
	defer d.configMu.RUnlock()
	return d.config
}

func (d *Daemon) preStartupValidation() error {
	if IsRunning() {
		pidPath, _ := PIDFile()
		return fmt.Errorf("daemon is already running (PID file exists at %s)", pidPath)
	}

	if err := d.config.Validate(); err != nil {
		return errors.WrapDaemon("validate configuration", err)
	}

	return nil
}

func (d *Daemon) setupResources(ctx context.Context) error {
	if err := d.writePIDFile(); err != nil {
		return errors.WrapDaemon("write PID file", err)
	}

	if err := d.processQueue(); err != nil {
		d.logger.Warn("queue processing encountered errors",
			slog.String("error", err.Error()))
	}

	return nil
}

func (d *Daemon) startServices(ctx context.Context) error {
	apiServer := api.NewServer(d.storage, d.getConfig)
	mux := apiServer.SetupRoutes()

	addr := fmt.Sprintf("127.0.0.1:%d", d.config.HTTP.Port)
	d.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	d.startPlugins(ctx)
	d.moduleCtx = ctx
	d.setupPollers()
	d.pollerManager.StartWithContext(ctx)

	if err := d.startConfigWatcher(ctx); err != nil {
		d.logger.Warn("failed to start config watcher",
			slog.String("error", err.Error()))
	}

	d.startQueueProcessor(ctx)
	d.startMetricsUpdater(ctx)

	return nil
}

func (d *Daemon) runEventLoop(ctx context.Context, cancel context.CancelFunc) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
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
		d.cleanupOnError()
		return fmt.Errorf("server error: %w", err)
	}
}

func (d *Daemon) cleanupOnError() {
	d.logger.Warn("startup failed, cleaning up resources")

	if d.pluginCtxCancel != nil {
		d.logger.Debug("stopping plugins")
		d.pluginCtxCancel()

		done := make(chan struct{})
		go func() {
			d.pluginWG.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(PluginShutdownTimeoutShort):
		}
	}

	if d.pollerManager != nil {
		d.logger.Debug("stopping pollers")
		d.pollerManager.Stop()
	}

	if d.server != nil {
		d.logger.Debug("stopping HTTP server")
		ctx, cancel := context.WithTimeout(context.Background(), ServerShutdownTimeoutShort)
		defer cancel()
		if err := d.server.Shutdown(ctx); err != nil {
			d.logger.Debug("error during server shutdown",
				slog.String("error", err.Error()))
		}
	}

	d.removePIDFile()
	d.logger.Debug("cleanup completed")
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
	filteredCount := 0
	for _, event := range queuedEvents {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := d.eventService.IngestEvent(ctx, event)
		cancel()

		if err == services.ErrEventFiltered || err == services.ErrDuplicateEvent {
			filteredCount++
			if err := q.Remove(event.ID); err != nil {
				d.logger.Warn("failed to remove filtered event from queue",
					slog.String("event_id", event.ID),
					slog.String("error", err.Error()))
			}
			continue
		}

		var validationErr *services.ValidationError
		if stderrors.As(err, &validationErr) {
			filteredCount++
			d.logger.Warn("removing invalid event from queue",
				slog.String("event_id", event.ID),
				slog.String("error", err.Error()))
			if err := q.Remove(event.ID); err != nil {
				d.logger.Warn("failed to remove invalid event from queue",
					slog.String("event_id", event.ID),
					slog.String("error", err.Error()))
			}
			continue
		}

		if err != nil {
			d.logger.Warn("failed to ingest queued event",
				slog.String("event_id", event.ID),
				slog.String("error", err.Error()))
			continue
		}

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
		slog.Int("filtered", filteredCount),
		slog.Int("total", len(queuedEvents)))
	return nil
}

func (d *Daemon) startQueueProcessor(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(QueueProcessorInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				d.logger.Debug("queue processor stopped")
				return
			case <-ticker.C:
				if err := d.processQueue(); err != nil {
					d.logger.Debug("queue processing error",
						slog.String("error", err.Error()))
				}
			}
		}
	}()
}

func (d *Daemon) startMetricsUpdater(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(MetricsUpdaterInterval)
		defer ticker.Stop()

		updateMetrics := func() {
			queueDir, err := config.QueueDir()
			if err != nil {
				d.logger.Debug("failed to get queue directory",
					slog.String("error", err.Error()))
				return
			}

			q, err := queue.New(queueDir)
			if err != nil {
				d.logger.Debug("failed to create queue",
					slog.String("error", err.Error()))
				return
			}

			queueDepth, err := q.Count()
			if err != nil {
				d.logger.Debug("failed to count queue",
					slog.String("error", err.Error()))
				queueDepth = 0
			}

			eventCount, err := d.storage.Count()
			if err != nil {
				d.logger.Debug("failed to count events",
					slog.String("error", err.Error()))
				eventCount = 0
			}

			var dbSize int64
			dataDir, err := config.DataDir()
			if err == nil {
				dbPath := filepath.Join(dataDir, "events.db")
				if stat, err := os.Stat(dbPath); err == nil {
					dbSize = stat.Size()
				}
			}

			metrics.GlobalSnapshot.UpdateSystemMetrics(int64(queueDepth), dbSize, int64(eventCount))
		}

		updateMetrics()

		for {
			select {
			case <-ctx.Done():
				d.logger.Debug("metrics updater stopped")
				return
			case <-ticker.C:
				updateMetrics()
			}
		}
	}()
}

func (d *Daemon) Shutdown() error {
	d.logger.Info("shutting down daemon")

	select {
	case <-d.stopChan:
		return nil
	default:
		close(d.stopChan)
	}

	if d.configWatcher != nil {
		d.logger.Debug("stopping config watcher")
		if err := d.configWatcher.Close(); err != nil {
			d.logger.Warn("error closing config watcher",
				slog.String("error", err.Error()))
		}
	}

	if d.pluginCtxCancel != nil {
		d.logger.Debug("stopping plugins")
		d.pluginCtxCancel()

		done := make(chan struct{})
		go func() {
			d.pluginWG.Wait()
			close(done)
		}()

		select {
		case <-done:
			d.logger.Debug("plugins stopped")
		case <-time.After(PluginShutdownTimeout):
			d.logger.Warn("plugins did not stop within timeout")
		}
	}

	if d.pollerManager != nil {
		d.pollerManager.Stop()
		d.logger.Debug("poller manager stopped")
	}

	ctx, cancel := context.WithTimeout(context.Background(), ServerShutdownTimeout)
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

	if d.logger != nil {
		if err := d.logger.Close(); err != nil {
			d.logger.Error("failed to close logger", slog.String("error", err.Error()))
		}
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

	for i := 0; i < StopDaemonMaxAttempts; i++ {
		if !IsRunning() {
			pidPath, _ := PIDFile()
			os.Remove(pidPath)
			return nil
		}
		time.Sleep(StopDaemonPollInterval)
	}

	return fmt.Errorf("daemon did not stop after %d attempts", StopDaemonMaxAttempts)
}

func SpawnBackground() *exec.Cmd {
	executable, err := os.Executable()
	if err != nil {
		executable = "devlog"
	}

	dataDir, _ := config.DataDir()
	logPath := filepath.Join(dataDir, "daemon.log")

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Cannot open daemon.log: %v\n", err)
		logFile = nil
	}

	cmd := exec.Command(executable, "daemon", "start")
	cmd.Env = append(os.Environ(), "DEVLOG_DAEMON_SUBPROCESS=1")

	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		cmd.ExtraFiles = []*os.File{logFile}
	} else {
		cmd.Stdout = nil
		cmd.Stderr = nil
	}

	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	return cmd
}
