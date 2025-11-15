package daemon

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"devlog/internal/api"
	"devlog/internal/config"
	"devlog/internal/events"
	"devlog/internal/queue"
	"devlog/internal/session"
	"devlog/internal/storage"
	"devlog/modules/wisprflow"
)

type Daemon struct {
	config         *config.Config
	storage        *storage.Storage
	sessionManager *session.Manager
	server         *http.Server
	stopChan       chan struct{}
}

func New(cfg *config.Config, store *storage.Storage) *Daemon {
	sessionManager := session.NewManager(store)

	return &Daemon{
		config:         cfg,
		storage:        store,
		sessionManager: sessionManager,
		stopChan:       make(chan struct{}),
	}
}

func (d *Daemon) Start() error {
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
		fmt.Fprintf(os.Stderr, "Warning: failed to process queue: %v\n", err)
	}

	if d.config.IsModuleEnabled("wisprflow") {
		go d.pollWisprFlow()
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		fmt.Printf("devlogd started on %s\n", addr)
		if err := d.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case <-sigChan:
		fmt.Println("\nShutting down gracefully...")
		return d.Shutdown()
	case err := <-errChan:
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

	fmt.Printf("Processing %d queued event(s) from disk...\n", len(queuedEvents))

	successCount := 0
	for _, event := range queuedEvents {
		if err := event.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping invalid queued event %s: %v\n", event.ID, err)
			continue
		}

		if err := d.storage.InsertEvent(event); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to insert queued event %s: %v\n", event.ID, err)
			continue
		}

		if err := q.Remove(event.ID); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove event %s from queue: %v\n", event.ID, err)
		} else {
			successCount++
		}
	}

	fmt.Printf("Processed %d/%d queued events\n", successCount, len(queuedEvents))
	return nil
}

func (d *Daemon) pollWisprFlow() {
	modCfg, ok := d.config.GetModuleConfig("wisprflow")
	if !ok {
		fmt.Fprintln(os.Stderr, "Warning: wisprflow module config not found")
		return
	}

	pollInterval := 60.0
	if interval, ok := modCfg["poll_interval_seconds"].(float64); ok {
		pollInterval = interval
	}

	minWords := 0.0
	if mw, ok := modCfg["min_words"].(float64); ok {
		minWords = mw
	}

	dbPathConfig, _ := modCfg["db_path"].(string)
	homeDir, _ := os.UserHomeDir()
	dbPath := wisprflow.GetDBPath(homeDir, dbPathConfig)

	dataDir, err := config.DataDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: wisprflow polling failed to get data dir: %v\n", err)
		return
	}

	ticker := time.NewTicker(time.Duration(pollInterval) * time.Second)
	defer ticker.Stop()

	fmt.Printf("Wispr Flow polling started (interval: %.0fs)\n", pollInterval)

	d.doWisprFlowPoll(dbPath, dataDir, minWords)

	for {
		select {
		case <-ticker.C:
			d.doWisprFlowPoll(dbPath, dataDir, minWords)
		case <-d.stopChan:
			return
		}
	}
}

func (d *Daemon) doWisprFlowPoll(dbPath string, dataDir string, minWords float64) {
	lastPoll, err := wisprflow.LoadLastPollTime(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: wisprflow poll failed to load timestamp: %v\n", err)
		return
	}

	entries, err := wisprflow.PollDatabase(dbPath, lastPoll)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: wisprflow poll failed: %v\n", err)
		return
	}

	if len(entries) == 0 {
		return
	}

	fmt.Printf("Wispr Flow: found %d new transcription(s)\n", len(entries))

	storedCount := 0
	for _, entry := range entries {
		if minWords > 0 && float64(entry.NumWords) < minWords {
			continue
		}

		text := entry.EditedText
		if text == "" {
			text = entry.FormattedText
		}
		if text == "" {
			text = entry.ASRText
		}

		event := events.NewEvent("wisprflow", "transcription")
		event.ID = entry.TranscriptEntityID
		event.Timestamp = entry.Timestamp.Format(time.RFC3339)
		event.Payload = map[string]interface{}{
			"id":             entry.TranscriptEntityID,
			"text":           text,
			"asr_text":       entry.ASRText,
			"formatted_text": entry.FormattedText,
			"edited_text":    entry.EditedText,
			"app":            entry.App,
			"url":            entry.URL,
			"duration":       entry.Duration,
			"num_words":      entry.NumWords,
			"status":         entry.Status,
		}

		if err := d.storage.InsertEvent(event); err != nil {
			if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
				fmt.Fprintf(os.Stderr, "Warning: failed to store wisprflow event: %v\n", err)
			}
		} else {
			storedCount++
		}
	}

	if len(entries) > 0 {
		lastEntry := entries[len(entries)-1]
		nextPollTime := lastEntry.Timestamp.Add(1 * time.Millisecond)
		if err := wisprflow.SaveLastPollTime(dataDir, nextPollTime); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save poll timestamp: %v\n", err)
		}
	}

	if storedCount > 0 {
		fmt.Printf("Wispr Flow: stored %d event(s)\n", storedCount)
	}
}

func (d *Daemon) Shutdown() error {
	close(d.stopChan)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if d.server != nil {
		if err := d.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
	}

	if d.storage != nil {
		if err := d.storage.Close(); err != nil {
			return fmt.Errorf("close storage: %w", err)
		}
	}

	d.removePIDFile()
	fmt.Println("Daemon stopped")
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
