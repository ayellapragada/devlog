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
	"devlog/internal/queue"
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
	case "flush":
		return flushCommand()
	case "init":
		return initCommand()
	case "session":
		return sessionCommand()
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
	fmt.Println("  devlog flush                         Process queued events")
	fmt.Println("  devlog status                        Show recent events")
	fmt.Println("  devlog session create --events <ids> Create session from event IDs")
	fmt.Println("  devlog session list                  List all sessions")
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
	// Load config to get port
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Try to send to daemon if running
	if daemon.IsRunning() {
		// Serialize event
		eventJSON, err := event.ToJSON()
		if err != nil {
			return fmt.Errorf("serialize event: %w", err)
		}

		// Send to daemon
		url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/ingest", cfg.HTTP.Port)
		resp, err := http.Post(url, "application/json", bytes.NewReader(eventJSON))
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("daemon returned error: %s", string(body))
		}
		// If we get here, daemon is running but unreachable - fall through to queue
	}

	// Daemon not running or unreachable - queue the event
	queueDir, err := config.QueueDir()
	if err != nil {
		return fmt.Errorf("get queue directory: %w", err)
	}

	q, err := queue.New(queueDir)
	if err != nil {
		return fmt.Errorf("create queue: %w", err)
	}

	if err := q.Enqueue(event); err != nil {
		return fmt.Errorf("queue event: %w", err)
	}

	// Silently queued - return success
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
		fmt.Printf("[%s] ", ts.Format("2006-01-02 15:04:05"))

		// Format based on event type
		switch event.Type {
		case "commit":
			formatCommitEvent(event)
		case "note":
			formatNoteEvent(event)
		case "merge":
			formatMergeEvent(event)
		case "command":
			formatCommandEvent(event)
		case "pr_merged":
			formatPREvent(event)
		default:
			// Fallback for unknown types
			fmt.Printf("%s/%s", event.Source, event.Type)
			if event.Repo != "" {
				fmt.Printf(" repo=%s", filepath.Base(event.Repo))
			}
		}

		fmt.Println()
	}

	// Show total count
	count, _ := store.Count()
	fmt.Printf("\nTotal events: %d\n", count)

	return nil
}

func formatCommitEvent(event *events.Event) {
	fmt.Printf("Git commit")
	if event.Repo != "" {
		fmt.Printf(" in %s", filepath.Base(event.Repo))
	}
	if event.Branch != "" {
		fmt.Printf(" [%s]", event.Branch)
	}
	if hash, ok := event.Payload["hash"].(string); ok {
		if len(hash) > 7 {
			hash = hash[:7]
		}
		fmt.Printf(" (%s)", hash)
	}
	if message, ok := event.Payload["message"].(string); ok {
		// Show first line only
		if idx := bytes.IndexByte([]byte(message), '\n'); idx != -1 {
			message = message[:idx]
		}
		if len(message) > 60 {
			message = message[:60] + "..."
		}
		fmt.Printf(": %s", message)
	}
}

func formatNoteEvent(event *events.Event) {
	fmt.Printf("Note")
	if text, ok := event.Payload["text"].(string); ok {
		// Show first line only
		if idx := bytes.IndexByte([]byte(text), '\n'); idx != -1 {
			text = text[:idx]
		}
		if len(text) > 80 {
			text = text[:80] + "..."
		}
		fmt.Printf(": %s", text)
	} else {
		fmt.Printf(" (empty)")
	}
}

func formatMergeEvent(event *events.Event) {
	fmt.Printf("Git merge")
	if event.Repo != "" {
		fmt.Printf(" in %s", filepath.Base(event.Repo))
	}
	if event.Branch != "" {
		fmt.Printf(" into %s", event.Branch)
	}
	if source, ok := event.Payload["source_branch"].(string); ok {
		fmt.Printf(" from %s", source)
	}
}

func formatCommandEvent(event *events.Event) {
	fmt.Printf("Shell command")
	if cmd, ok := event.Payload["command"].(string); ok {
		if len(cmd) > 80 {
			cmd = cmd[:80] + "..."
		}
		fmt.Printf(": %s", cmd)
	}
	if event.Repo != "" {
		fmt.Printf(" (in %s)", filepath.Base(event.Repo))
	}
}

func formatPREvent(event *events.Event) {
	fmt.Printf("GitHub PR merged")
	if title, ok := event.Payload["title"].(string); ok {
		if len(title) > 60 {
			title = title[:60] + "..."
		}
		fmt.Printf(": %s", title)
	}
	if prNum, ok := event.Payload["pr_number"].(float64); ok {
		fmt.Printf(" (#%.0f)", prNum)
	}
	if event.Repo != "" {
		fmt.Printf(" in %s", filepath.Base(event.Repo))
	}
}

func flushCommand() error {
	// Check if daemon is running
	if !daemon.IsRunning() {
		return fmt.Errorf("daemon is not running (start it with 'devlog daemon start')")
	}

	// Get queue directory
	queueDir, err := config.QueueDir()
	if err != nil {
		return fmt.Errorf("get queue directory: %w", err)
	}

	// Open queue
	q, err := queue.New(queueDir)
	if err != nil {
		return fmt.Errorf("open queue: %w", err)
	}

	// Get queued events
	queuedEvents, err := q.List()
	if err != nil {
		return fmt.Errorf("list queue: %w", err)
	}

	if len(queuedEvents) == 0 {
		fmt.Println("No queued events")
		return nil
	}

	fmt.Printf("Processing %d queued event(s)...\n", len(queuedEvents))

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Send each event to daemon
	successCount := 0
	for _, event := range queuedEvents {
		eventJSON, err := event.ToJSON()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to serialize event %s: %v\n", event.ID, err)
			continue
		}

		url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/ingest", cfg.HTTP.Port)
		resp, err := http.Post(url, "application/json", bytes.NewReader(eventJSON))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to send event %s: %v\n", event.ID, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "Warning: daemon rejected event %s (status %d)\n", event.ID, resp.StatusCode)
			continue
		}

		// Successfully sent - remove from queue
		if err := q.Remove(event.ID); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove event %s from queue: %v\n", event.ID, err)
		} else {
			successCount++
		}
	}

	fmt.Printf("Successfully processed %d/%d events\n", successCount, len(queuedEvents))

	return nil
}

func sessionCommand() error {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  devlog session create --events <id1> <id2> ... [--description <text>]")
		fmt.Println("  devlog session list")
		return fmt.Errorf("missing session subcommand")
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "create":
		return sessionCreate()
	case "list":
		return sessionList()
	default:
		fmt.Fprintf(os.Stderr, "Unknown session subcommand: %s\n", subcommand)
		return fmt.Errorf("unknown session subcommand: %s", subcommand)
	}
}

func sessionCreate() error {
	fs := flag.NewFlagSet("session-create", flag.ExitOnError)
	description := fs.String("description", "", "Session description")

	// Parse flags
	fs.Parse(os.Args[3:])

	// Remaining args are event IDs
	eventIDs := fs.Args()

	if len(eventIDs) == 0 {
		return fmt.Errorf("at least one event ID is required")
	}

	// Check if daemon is running
	if !daemon.IsRunning() {
		return fmt.Errorf("daemon is not running (start it with 'devlog daemon start')")
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Prepare request
	reqBody := map[string]interface{}{
		"event_ids":   eventIDs,
		"description": *description,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	// Send to daemon
	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/sessions", cfg.HTTP.Port)
	resp, err := http.Post(url, "application/json", bytes.NewReader(reqJSON))
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon returned error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	sessionID, _ := result["session_id"].(string)
	eventCount, _ := result["event_count"].(float64)

	fmt.Printf("Session created: %s\n", sessionID)
	if *description != "" {
		fmt.Printf("Description: %s\n", *description)
	}
	fmt.Printf("Events: %.0f\n", eventCount)

	return nil
}

func sessionList() error {
	// Check if daemon is running
	if !daemon.IsRunning() {
		return fmt.Errorf("daemon is not running (start it with 'devlog daemon start')")
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Get sessions from daemon
	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/sessions", cfg.HTTP.Port)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon returned error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	sessions, ok := result["sessions"].([]interface{})
	if !ok || len(sessions) == 0 {
		fmt.Println("No sessions yet")
		return nil
	}

	fmt.Printf("Sessions (%d total):\n\n", len(sessions))

	for _, s := range sessions {
		sess, ok := s.(map[string]interface{})
		if !ok {
			continue
		}

		sessionID, _ := sess["id"].(string)
		description, _ := sess["description"].(string)
		eventCount, _ := sess["event_count"].(float64)
		startTimeStr, _ := sess["start_time"].(string)
		duration, _ := sess["duration"].(string)

		fmt.Printf("ID: %s\n", sessionID)
		if startTimeStr != "" {
			ts, err := time.Parse(time.RFC3339, startTimeStr)
			if err == nil {
				fmt.Printf("  Started: %s\n", ts.Local().Format("2006-01-02 15:04:05"))
			}
		}
		if description != "" {
			fmt.Printf("  Description: %s\n", description)
		}
		fmt.Printf("  Events: %.0f\n", eventCount)
		if duration != "" {
			fmt.Printf("  Duration: %s\n", duration)
		}
		fmt.Println()
	}

	return nil
}
