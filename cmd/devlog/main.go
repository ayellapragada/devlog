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
	"devlog/internal/modules"
	"devlog/internal/queue"
	"devlog/internal/storage"

	// Import modules to register them
	_ "devlog/modules/git"
	_ "devlog/modules/shell"
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
	case "config":
		return configCommand()
	case "module":
		return moduleCommand()
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
	fmt.Println("  devlog config status                 Show configuration status")
	fmt.Println("  devlog module list                   List available modules")
	fmt.Println("  devlog module install <name>         Install and enable a module")
	fmt.Println("  devlog module uninstall <name>       Uninstall and disable a module")
	fmt.Println("  devlog daemon start                  Start the daemon")
	fmt.Println("  devlog daemon stop                   Stop the daemon")
	fmt.Println("  devlog daemon status                 Check daemon status")
	fmt.Println("  devlog ingest git-commit [flags]     Ingest a git commit event")
	fmt.Println("  devlog ingest shell-command [flags]  Ingest a shell command event")
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

	fmt.Println()
	fmt.Println("  2. Install modules to enable event capture:")

	// List available modules
	allModules := modules.List()
	for _, mod := range allModules {
		fmt.Printf("     - %s: %s\n", mod.Name(), mod.Description())
	}

	fmt.Println()
	fmt.Println("     Install modules with: devlog module install <name>")
	fmt.Println()
	fmt.Println("  3. Start the daemon:")
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
		fmt.Println("Usage:")
		fmt.Println("  devlog ingest git-commit [flags]")
		fmt.Println("  devlog ingest shell-command [flags]")
		return fmt.Errorf("missing ingest subcommand")
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "git-commit":
		return ingestGitCommit()
	case "shell-command":
		return ingestShellCommand()
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

func ingestShellCommand() error {
	fs := flag.NewFlagSet("shell-command", flag.ExitOnError)
	command := fs.String("command", "", "The shell command")
	exitCode := fs.Int("exit-code", 0, "Command exit code")
	workdir := fs.String("workdir", "", "Working directory")
	duration := fs.Int64("duration", 0, "Command duration in milliseconds")

	fs.Parse(os.Args[3:])

	if *command == "" {
		return fmt.Errorf("--command is required")
	}

	// Create event
	event := events.NewEvent(events.SourceShell, events.TypeCommand)
	event.Payload["command"] = *command
	event.Payload["exit_code"] = *exitCode

	if *workdir != "" {
		event.Payload["workdir"] = *workdir
		// Check if workdir is a git repo
		if repoPath, err := findGitRepo(*workdir); err == nil {
			event.Repo = repoPath
		}
	}

	if *duration > 0 {
		event.Payload["duration_ms"] = *duration
	}

	// Send to daemon
	return sendEvent(event)
}

func findGitRepo(path string) (string, error) {
	// This is a simple helper - in production you might want to use git commands
	current := path
	for {
		gitPath := filepath.Join(current, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("not a git repository")
		}
		current = parent
	}
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
		formatEvent(event)
	}

	// Show total count
	count, _ := store.Count()
	fmt.Printf("\nTotal events: %d\n", count)

	return nil
}

// formatEvent formats an event for display in a consistent, scannable format
// Format: [TIMESTAMP] (type) folder: description [metadata]
func formatEvent(event *events.Event) {
	// Parse and format timestamp
	ts, _ := time.Parse(time.RFC3339, event.Timestamp)
	fmt.Printf("[%s] ", ts.Format("2006-01-02 15:04:05"))

	// Event type tag
	typeTag := getTypeTag(event)
	fmt.Printf("(%s) ", typeTag)

	// Folder/repo name
	folder := getFolder(event)
	if folder != "" {
		fmt.Printf("%s: ", folder)
	}

	// Main content
	switch event.Type {
	case "commit":
		formatCommitContent(event)
	case "merge":
		formatMergeContent(event)
	case "command":
		formatCommandContent(event)
	case "note":
		formatNoteContent(event)
	case "pr_merged":
		formatPRContent(event)
	default:
		fmt.Printf("%s/%s", event.Source, event.Type)
	}

	fmt.Println()
}

// getTypeTag returns a short type tag for the event
func getTypeTag(event *events.Event) string {
	switch event.Type {
	case "commit":
		return "git"
	case "merge":
		return "git"
	case "command":
		return "shell"
	case "note":
		return "note"
	case "pr_merged":
		return "github"
	default:
		return event.Type
	}
}

// getFolder returns the folder/repo name for display
func getFolder(event *events.Event) string {
	if event.Repo != "" {
		return filepath.Base(event.Repo)
	}
	// For shell commands without repo, try to get from workdir
	if event.Type == "command" {
		if workdir, ok := event.Payload["workdir"].(string); ok {
			return filepath.Base(workdir)
		}
	}
	return ""
}

// formatCommitContent formats commit event content
func formatCommitContent(event *events.Event) {
	// Get commit message
	message := ""
	if msg, ok := event.Payload["message"].(string); ok {
		// First line only
		if idx := bytes.IndexByte([]byte(msg), '\n'); idx != -1 {
			message = msg[:idx]
		} else {
			message = msg
		}
		// Truncate if too long
		if len(message) > 60 {
			message = message[:60] + "..."
		}
	}

	fmt.Printf("%s", message)

	// Add metadata: [branch@hash]
	var metadata []string
	if event.Branch != "" {
		metadata = append(metadata, event.Branch)
	}
	if hash, ok := event.Payload["hash"].(string); ok {
		if len(hash) > 7 {
			hash = hash[:7]
		}
		if len(metadata) > 0 {
			fmt.Printf(" [%s@%s]", metadata[0], hash)
		} else {
			fmt.Printf(" [%s]", hash)
		}
	} else if len(metadata) > 0 {
		fmt.Printf(" [%s]", metadata[0])
	}
}

// formatMergeContent formats merge event content
func formatMergeContent(event *events.Event) {
	source := ""
	if src, ok := event.Payload["source_branch"].(string); ok {
		source = src
	}

	target := event.Branch
	if target == "" {
		target = "?"
	}

	if source != "" {
		fmt.Printf("merge %s → %s", source, target)
	} else {
		fmt.Printf("merge → %s", target)
	}
}

// formatCommandContent formats shell command event content
func formatCommandContent(event *events.Event) {
	cmd := ""
	if c, ok := event.Payload["command"].(string); ok {
		cmd = c
		// Truncate if too long
		if len(cmd) > 70 {
			cmd = cmd[:70] + "..."
		}
	}

	fmt.Printf("%s", cmd)

	// Add exit code if non-zero
	if exitCode, ok := event.Payload["exit_code"].(float64); ok && exitCode != 0 {
		fmt.Printf(" [exit:%d]", int(exitCode))
	} else if exitCode, ok := event.Payload["exit_code"].(int); ok && exitCode != 0 {
		fmt.Printf(" [exit:%d]", exitCode)
	}

	// Add duration if available
	if duration, ok := event.Payload["duration_ms"].(float64); ok && duration > 0 {
		fmt.Printf(" [%s]", formatDurationMs(int64(duration)))
	} else if duration, ok := event.Payload["duration_ms"].(int64); ok && duration > 0 {
		fmt.Printf(" [%s]", formatDurationMs(duration))
	}
}

// formatNoteContent formats note event content
func formatNoteContent(event *events.Event) {
	text := ""
	if t, ok := event.Payload["text"].(string); ok {
		// First line only
		if idx := bytes.IndexByte([]byte(t), '\n'); idx != -1 {
			text = t[:idx]
		} else {
			text = t
		}
		// Truncate if too long
		if len(text) > 80 {
			text = text[:80] + "..."
		}
	}

	if text != "" {
		fmt.Printf("%s", text)
	} else {
		fmt.Printf("(empty)")
	}
}

// formatPRContent formats PR merged event content
func formatPRContent(event *events.Event) {
	title := ""
	if t, ok := event.Payload["title"].(string); ok {
		title = t
		if len(title) > 60 {
			title = title[:60] + "..."
		}
	}

	prNum := ""
	if num, ok := event.Payload["pr_number"].(float64); ok {
		prNum = fmt.Sprintf("#%.0f", num)
	}

	if prNum != "" && title != "" {
		fmt.Printf("%s: %s", prNum, title)
	} else if prNum != "" {
		fmt.Printf("%s", prNum)
	} else if title != "" {
		fmt.Printf("%s", title)
	}
}

// formatDurationMs formats milliseconds into a human-readable duration
func formatDurationMs(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	seconds := ms / 1000
	if seconds < 60 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	minutes := seconds / 60
	seconds = seconds % 60
	return fmt.Sprintf("%dm%ds", minutes, seconds)
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

func configCommand() error {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  devlog config status")
		return fmt.Errorf("missing config subcommand")
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "status":
		return configStatus()
	default:
		fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n", subcommand)
		return fmt.Errorf("unknown config subcommand: %s", subcommand)
	}
}

func configStatus() error {
	// Try to load config
	cfg, err := config.Load()
	if err != nil {
		// Check if config doesn't exist yet
		configPath, _ := config.ConfigPath()
		if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
			fmt.Println("Configuration Status")
			fmt.Println("===================")
			fmt.Println()
			fmt.Println("Status: Not initialized")
			fmt.Println()
			fmt.Println("Run 'devlog init' to initialize devlog")
			return nil
		}
		return err
	}

	configPath, _ := config.ConfigPath()
	dataDir, _ := config.DataDir()

	fmt.Println("Configuration Status")
	fmt.Println("===================")
	fmt.Println()
	fmt.Printf("Config file: %s\n", configPath)
	fmt.Printf("Data directory: %s\n", dataDir)
	fmt.Printf("Obsidian path: %s\n", cfg.ObsidianPath)
	fmt.Printf("HTTP port: %d\n", cfg.HTTP.Port)
	fmt.Println()

	// List modules and their status
	fmt.Println("Modules:")
	allModules := modules.List()
	if len(allModules) == 0 {
		fmt.Println("  No modules available")
	} else {
		for _, mod := range allModules {
			enabled := cfg.IsModuleEnabled(mod.Name())
			status := "disabled"
			if enabled {
				status = "enabled"
			}
			fmt.Printf("  [%s] %s - %s\n", status, mod.Name(), mod.Description())
		}
	}

	fmt.Println()
	fmt.Println("Use 'devlog module install <name>' to enable a module")

	return nil
}

func moduleCommand() error {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  devlog module list")
		fmt.Println("  devlog module install <name>")
		fmt.Println("  devlog module uninstall <name>")
		return fmt.Errorf("missing module subcommand")
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "list":
		return moduleList()
	case "install", "init":
		return moduleInstall()
	case "uninstall":
		return moduleUninstall()
	default:
		fmt.Fprintf(os.Stderr, "Unknown module subcommand: %s\n", subcommand)
		return fmt.Errorf("unknown module subcommand: %s", subcommand)
	}
}

func moduleList() error {
	fmt.Println("Available Modules")
	fmt.Println("================")
	fmt.Println()

	allModules := modules.List()
	if len(allModules) == 0 {
		fmt.Println("No modules available")
		return nil
	}

	// Try to load config to show status
	cfg, err := config.Load()
	var showStatus bool
	if err == nil {
		showStatus = true
	}

	for _, mod := range allModules {
		status := ""
		if showStatus {
			if cfg.IsModuleEnabled(mod.Name()) {
				status = " [enabled]"
			} else {
				status = " [disabled]"
			}
		}
		fmt.Printf("  %s%s\n", mod.Name(), status)
		fmt.Printf("    %s\n", mod.Description())
		fmt.Println()
	}

	if !showStatus {
		fmt.Println("Run 'devlog init' to initialize configuration")
	}

	return nil
}

func moduleInstall() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: devlog module install <name>")
	}

	moduleName := os.Args[3]

	// Get the module
	mod, err := modules.Get(moduleName)
	if err != nil {
		return fmt.Errorf("module not found: %s", moduleName)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w (run 'devlog init' first)", err)
	}

	// Check if already enabled
	if cfg.IsModuleEnabled(moduleName) {
		fmt.Printf("Module '%s' is already enabled\n", moduleName)
		return nil
	}

	fmt.Printf("Installing module: %s\n", moduleName)
	fmt.Printf("Description: %s\n", mod.Description())
	fmt.Println()

	// Create install context
	homeDir, _ := os.UserHomeDir()
	configDir, _ := config.ConfigDir()
	dataDir, _ := config.DataDir()

	ctx := &modules.InstallContext{
		Interactive: true,
		ConfigDir:   configDir,
		DataDir:     dataDir,
		HomeDir:     homeDir,
		Log: func(format string, args ...interface{}) {
			fmt.Printf(format+"\n", args...)
		},
	}

	// Install the module
	if err := mod.Install(ctx); err != nil {
		return fmt.Errorf("install module: %w", err)
	}

	// Enable in config
	cfg.SetModuleEnabled(moduleName, true)

	// Set default config for the module
	defaultCfg := mod.DefaultConfig()
	if cfgMap, ok := defaultCfg.(map[string]interface{}); ok {
		cfg.SetModuleConfig(moduleName, cfgMap)
	}

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println()
	fmt.Printf("✓ Module '%s' installed and enabled\n", moduleName)

	return nil
}

func moduleUninstall() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: devlog module uninstall <name>")
	}

	moduleName := os.Args[3]

	// Get the module
	mod, err := modules.Get(moduleName)
	if err != nil {
		return fmt.Errorf("module not found: %s", moduleName)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Check if not enabled
	if !cfg.IsModuleEnabled(moduleName) {
		fmt.Printf("Module '%s' is not enabled\n", moduleName)
		return nil
	}

	fmt.Printf("Uninstalling module: %s\n", moduleName)
	fmt.Println()

	// Create install context
	homeDir, _ := os.UserHomeDir()
	configDir, _ := config.ConfigDir()
	dataDir, _ := config.DataDir()

	ctx := &modules.InstallContext{
		Interactive: true,
		ConfigDir:   configDir,
		DataDir:     dataDir,
		HomeDir:     homeDir,
		Log: func(format string, args ...interface{}) {
			fmt.Printf(format+"\n", args...)
		},
	}

	// Uninstall the module
	if err := mod.Uninstall(ctx); err != nil {
		return fmt.Errorf("uninstall module: %w", err)
	}

	// Disable in config
	cfg.SetModuleEnabled(moduleName, false)

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println()
	fmt.Printf("✓ Module '%s' uninstalled and disabled\n", moduleName)

	return nil
}
