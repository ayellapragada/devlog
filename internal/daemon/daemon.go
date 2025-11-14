package daemon

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"devlog/internal/api"
	"devlog/internal/config"
	"devlog/internal/storage"
)

// Daemon manages the devlogd lifecycle
type Daemon struct {
	config  *config.Config
	storage *storage.Storage
	server  *http.Server
}

// New creates a new Daemon instance
func New(cfg *config.Config, store *storage.Storage) *Daemon {
	return &Daemon{
		config:  cfg,
		storage: store,
	}
}

// Start starts the daemon HTTP server
func (d *Daemon) Start() error {
	apiServer := api.NewServer(d.storage)
	mux := apiServer.SetupRoutes()

	addr := fmt.Sprintf("127.0.0.1:%d", d.config.HTTP.Port)
	d.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Write PID file
	if err := d.writePIDFile(); err != nil {
		return fmt.Errorf("write PID file: %w", err)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		fmt.Printf("devlogd started on %s\n", addr)
		if err := d.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for signal or error
	select {
	case <-sigChan:
		fmt.Println("\nShutting down gracefully...")
		return d.Shutdown()
	case err := <-errChan:
		d.removePIDFile()
		return fmt.Errorf("server error: %w", err)
	}
}

// Shutdown gracefully shuts down the daemon
func (d *Daemon) Shutdown() error {
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

// PIDFile returns the path to the PID file
func PIDFile() (string, error) {
	configDir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "devlogd.pid"), nil
}

// writePIDFile writes the current process ID to the PID file
func (d *Daemon) writePIDFile() error {
	pidPath, err := PIDFile()
	if err != nil {
		return err
	}

	// Check if daemon is already running
	if IsRunning() {
		return fmt.Errorf("daemon is already running (PID file exists at %s)", pidPath)
	}

	pid := os.Getpid()
	return os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644)
}

// removePIDFile removes the PID file
func (d *Daemon) removePIDFile() {
	pidPath, err := PIDFile()
	if err != nil {
		return
	}
	os.Remove(pidPath)
}

// IsRunning checks if the daemon is running
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

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds, so we need to send signal 0
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// GetPID returns the PID of the running daemon, or 0 if not running
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

// StopDaemon stops a running daemon
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

	// Wait for process to exit (with timeout)
	for i := 0; i < 50; i++ {
		if !IsRunning() {
			// Clean up PID file if it still exists
			pidPath, _ := PIDFile()
			os.Remove(pidPath)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("daemon did not stop after 5 seconds")
}
