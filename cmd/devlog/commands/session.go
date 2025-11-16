package commands

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"devlog/internal/config"
	"devlog/internal/daemon"
)

func init() {
	RegisterCommand("session", &CommandDefinition{
		Name:        "session",
		Description: "Manage development sessions",
		Usage:       "devlog session <subcommand>",
		Subcommands: map[string]*CommandDefinition{
			"create": {
				Name:        "create",
				Description: "Create a new session from event IDs",
				Usage:       "devlog session create --events <id1> <id2> ... [--description <text>]",
				Flags: []FlagDefinition{
					{Long: "--description", Description: "Optional description for the session"},
				},
				Examples: []string{
					"devlog session create --events abc123 def456",
					"devlog session create --events abc123 --description \"Bug fix session\"",
				},
			},
			"list": {
				Name:        "list",
				Description: "List all sessions",
				Usage:       "devlog session list",
				Examples: []string{
					"devlog session list",
				},
			},
		},
	})
}

func Session() error {
	if len(os.Args) < 3 {
		ShowHelp([]string{"session"})
		return fmt.Errorf("missing session subcommand")
	}

	subcommand := os.Args[2]

	if subcommand == "help" {
		ShowHelp([]string{"session"})
		return nil
	}

	switch subcommand {
	case "create":
		return sessionCreate()
	case "list":
		if len(os.Args) > 3 && os.Args[3] == "help" {
			ShowHelp([]string{"session", "list"})
			return nil
		}
		return sessionList()
	default:
		fmt.Fprintf(os.Stderr, "Unknown session subcommand: %s\n\n", subcommand)
		ShowHelp([]string{"session"})
		return fmt.Errorf("unknown session subcommand: %s", subcommand)
	}
}

func sessionCreate() error {
	if len(os.Args) > 3 && os.Args[3] == "help" {
		ShowHelp([]string{"session", "create"})
		return nil
	}

	fs := flag.NewFlagSet("session-create", flag.ExitOnError)
	description := fs.String("description", "", "Session description")

	fs.Parse(os.Args[3:])

	eventIDs := fs.Args()
	if len(eventIDs) == 0 {
		return fmt.Errorf("at least one event ID is required")
	}

	if !daemon.IsRunning() {
		return fmt.Errorf("daemon is not running (start it with 'devlog daemon start')")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	reqBody := map[string]interface{}{
		"event_ids":   eventIDs,
		"description": *description,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

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
	if !daemon.IsRunning() {
		return fmt.Errorf("daemon is not running (start it with 'devlog daemon start')")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

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
