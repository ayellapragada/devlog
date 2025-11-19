//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"devlog/modules/claude"
)

func main() {
	homeDir, _ := os.UserHomeDir()
	projectsDir := filepath.Join(homeDir, ".claude", "projects")
	cwd, _ := os.Getwd()

	fmt.Printf("Projects dir: %s\n", projectsDir)
	fmt.Printf("Current working dir: %s\n", cwd)
	fmt.Println()

	p, err := claude.NewPoller(
		projectsDir,
		cwd,
		"/tmp",
		30*time.Second,
		true,
		true,
		true,
		10,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Polling for events since 7 days ago...")
	ctx := context.Background()
	events, err := p.Poll(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nFound %d events:\n\n", len(events))
	for i, evt := range events {
		fmt.Printf("%d. [%s] %s/%s - %s\n", i+1, evt.Timestamp, evt.Source, evt.Type, evt.ID)
		if summary, ok := evt.Payload["summary"].(string); ok {
			fmt.Printf("   %s\n", summary)
		}
		if cmd, ok := evt.Payload["command"].(string); ok {
			fmt.Printf("   Command: %s\n", cmd)
		}
		if file, ok := evt.Payload["file_path"].(string); ok {
			fmt.Printf("   File: %s\n", file)
		}
		fmt.Println()
	}
}
