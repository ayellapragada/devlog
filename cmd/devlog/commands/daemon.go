package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"devlog/internal/config"
	"devlog/internal/daemon"
	"devlog/internal/storage"
)

func init() {
	RegisterCommand("daemon", &CommandDefinition{
		Name:        "daemon",
		Description: "Manage the devlog daemon process",
		Usage:       "devlog daemon <subcommand>",
		Subcommands: map[string]*CommandDefinition{
			"start": {
				Name:        "start",
				Description: "Start the devlog daemon",
				Usage:       "devlog daemon start",
				Examples: []string{
					"devlog daemon start",
				},
			},
			"stop": {
				Name:        "stop",
				Description: "Stop the devlog daemon",
				Usage:       "devlog daemon stop",
				Examples: []string{
					"devlog daemon stop",
				},
			},
			"restart": {
				Name:        "restart",
				Description: "Restart the devlog daemon",
				Usage:       "devlog daemon restart",
				Examples: []string{
					"devlog daemon restart",
				},
			},
			"status": {
				Name:        "status",
				Description: "Check the status of the devlog daemon",
				Usage:       "devlog daemon status",
				Examples: []string{
					"devlog daemon status",
				},
			},
		},
	})
}

func Daemon() error {
	if len(os.Args) < 3 {
		ShowHelp([]string{"daemon"})
		return fmt.Errorf("missing daemon subcommand")
	}

	subcommand := os.Args[2]

	if subcommand == "help" {
		ShowHelp([]string{"daemon"})
		return nil
	}

	switch subcommand {
	case "start":
		if len(os.Args) > 3 && os.Args[3] == "help" {
			ShowHelp([]string{"daemon", "start"})
			return nil
		}
		return daemonStart()
	case "stop":
		if len(os.Args) > 3 && os.Args[3] == "help" {
			ShowHelp([]string{"daemon", "stop"})
			return nil
		}
		return daemonStop()
	case "restart":
		if len(os.Args) > 3 && os.Args[3] == "help" {
			ShowHelp([]string{"daemon", "restart"})
			return nil
		}
		return daemonRestart()
	case "status":
		if len(os.Args) > 3 && os.Args[3] == "help" {
			ShowHelp([]string{"daemon", "status"})
			return nil
		}
		return daemonStatus()
	default:
		fmt.Fprintf(os.Stderr, "Unknown daemon subcommand: %s\n\n", subcommand)
		ShowHelp([]string{"daemon"})
		return fmt.Errorf("unknown daemon subcommand: %s", subcommand)
	}
}

func daemonStart() error {
	if daemon.IsRunning() {
		return fmt.Errorf("daemon is already running (PID %d)", daemon.GetPID())
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	dbPath := filepath.Join(dataDir, "events.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("database does not exist (run 'devlog init' first)")
	}

	store, err := storage.New(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	d := daemon.New(cfg, store)
	return d.Start()
}

func daemonStop() error {
	if !daemon.IsRunning() {
		fmt.Println("Daemon is not running")
		return nil
	}

	fmt.Printf("Stopping daemon (PID %d)...\n", daemon.GetPID())
	return daemon.StopDaemon()
}

func daemonRestart() error {
	if daemon.IsRunning() {
		fmt.Println("Stopping daemon...")
		if err := daemonStop(); err != nil {
			return fmt.Errorf("failed to stop daemon: %w", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("Starting daemon...")
	return daemonStart()
}

func daemonStatus() error {
	if daemon.IsRunning() {
		fmt.Printf("Daemon is running (PID %d)\n", daemon.GetPID())

		cfg, err := config.Load()
		if err == nil {
			url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/status", cfg.HTTP.Port)
			resp, err := http.Get(url)
			if err == nil {
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)

				var status map[string]interface{}
				if json.Unmarshal(body, &status) == nil {
					fmt.Printf("Event count: %v\n", status["event_count"])
					fmt.Printf("Uptime: %v seconds\n", status["uptime_seconds"])
				}
			}
		}
	} else {
		fmt.Println("Daemon is not running")
	}
	return nil
}
