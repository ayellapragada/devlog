package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"devlog/internal/config"
	"devlog/internal/daemon"
	"devlog/internal/events"
	"devlog/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		printUsage()
		return nil
	}

	command := os.Args[1]

	switch command {
	case "daemon":
		return daemonCommand()
	case "ingest":
		return ingestCommand()
	case "status":
		return statusCommand()
	case "init":
		return initCommand()
	case "help":
		printUsage()
		return nil
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		return fmt.Errorf("unknown command: %s", command)
	}
}

func printUsage() {
	fmt.Println("DevLog - Development journaling system")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  devlog init                          Initialize config and database")
	fmt.Println("  devlog daemon start                  Start the daemon")
	fmt.Println("  devlog daemon stop                   Stop the daemon")
	fmt.Println("  devlog daemon status                 Check daemon status")
	fmt.Println("  devlog ingest git-commit [flags]     Ingest a git commit event")
	fmt.Println("  devlog status                        Show recent events")
	fmt.Println("  devlog help                          Show this help message")
}

func initCommand() error {
	fmt.Println("Initializing devlog...")

	// Initialize config
	if err := config.InitConfig(); err != nil {
		return err
	}

	// Initialize database
	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	dbPath := filepath.Join(dataDir, "events.db")
	if err := storage.InitDB(dbPath); err != nil {
		return err
	}

	fmt.Println("\nInitialization complete!")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Edit your config file to set the Obsidian path")

	configPath, _ := config.ConfigPath()
	fmt.Printf("     %s\n", configPath)
	fmt.Println("  2. Start the daemon:")
	fmt.Println("     devlog daemon start")

	return nil
}

func daemonCommand() error {
	if len(os.Args) < 3 {
		fmt.Println("Usage: devlog daemon [start|stop|status]")
		return fmt.Errorf("missing daemon subcommand")
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "start":
		return daemonStart()
	case "stop":
		return daemonStop()
	case "status":
		return daemonStatus()
	default:
		fmt.Fprintf(os.Stderr, "Unknown daemon subcommand: %s\n", subcommand)
		return fmt.Errorf("unknown daemon subcommand: %s", subcommand)
	}
}

func daemonStart() error {
	if daemon.IsRunning() {
		return fmt.Errorf("daemon is already running (PID %d)", daemon.GetPID())
	}

	// Check config exists
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Check database exists
	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	dbPath := filepath.Join(dataDir, "events.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("database does not exist (run 'devlog init' first)")
	}

	// Open storage
	store, err := storage.New(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	// Create and start daemon
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

func daemonStatus() error {
	if daemon.IsRunning() {
		fmt.Printf("Daemon is running (PID %d)\n", daemon.GetPID())

		// Try to get status from API
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

func ingestCommand() error {
	if len(os.Args) < 3 {
		fmt.Println("Usage: devlog ingest git-commit [flags]")
		return fmt.Errorf("missing ingest subcommand")
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "git-commit":
		return ingestGitCommit()
	default:
		fmt.Fprintf(os.Stderr, "Unknown ingest subcommand: %s\n", subcommand)
		return fmt.Errorf("unknown ingest subcommand: %s", subcommand)
	}
}

func ingestGitCommit() error {
	fs := flag.NewFlagSet("git-commit", flag.ExitOnError)
	repo := fs.String("repo", "", "Repository path")
	branch := fs.String("branch", "", "Branch name")
	hash := fs.String("hash", "", "Commit hash")
	message := fs.String("message", "", "Commit message")
	author := fs.String("author", "", "Commit author")

	fs.Parse(os.Args[3:])

	if *repo == "" || *branch == "" || *hash == "" {
		return fmt.Errorf("--repo, --branch, and --hash are required")
	}

	// Create event
	event := events.NewEvent(events.SourceGit, events.TypeCommit)
	event.Repo = *repo
	event.Branch = *branch
	event.Payload["hash"] = *hash
	if *message != "" {
		event.Payload["message"] = *message
	}
	if *author != "" {
		event.Payload["author"] = *author
	}

	// Send to daemon
	return sendEvent(event)
}

func sendEvent(event *events.Event) error {
	// Check daemon is running
	if !daemon.IsRunning() {
		return fmt.Errorf("daemon is not running (start it with 'devlog daemon start')")
	}

	// Load config to get port
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Serialize event
	eventJSON, err := event.ToJSON()
	if err != nil {
		return fmt.Errorf("serialize event: %w", err)
	}

	// Send to daemon
	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/ingest", cfg.HTTP.Port)
	resp, err := http.Post(url, "application/json", bytes.NewReader(eventJSON))
	if err != nil {
		return fmt.Errorf("send event to daemon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon returned error: %s", string(body))
	}

	return nil
}

func statusCommand() error {
	// Load config
	_, err := config.Load()
	if err != nil {
		return err
	}

	// Get database path
	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	dbPath := filepath.Join(dataDir, "events.db")

	// Open storage
	store, err := storage.New(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	// List recent events
	recentEvents, err := store.ListEvents(10, "")
	if err != nil {
		return err
	}

	if len(recentEvents) == 0 {
		fmt.Println("No events yet")
		return nil
	}

	fmt.Printf("Recent events (showing last %d):\n\n", len(recentEvents))

	for _, event := range recentEvents {
		// Parse timestamp
		ts, _ := time.Parse(time.RFC3339, event.Timestamp)
		fmt.Printf("[%s] %s/%s", ts.Format("2006-01-02 15:04:05"), event.Source, event.Type)

		if event.Repo != "" {
			fmt.Printf(" repo=%s", filepath.Base(event.Repo))
		}

		if event.Branch != "" {
			fmt.Printf(" branch=%s", event.Branch)
		}

		// Show some payload data
		if hash, ok := event.Payload["hash"].(string); ok {
			fmt.Printf(" hash=%s", hash[:7])
		}

		if message, ok := event.Payload["message"].(string); ok {
			if len(message) > 50 {
				message = message[:50] + "..."
			}
			fmt.Printf(" \"%s\"", message)
		}

		fmt.Println()
	}

	// Show total count
	count, _ := store.Count()
	fmt.Printf("\nTotal events: %d\n", count)

	return nil
}
