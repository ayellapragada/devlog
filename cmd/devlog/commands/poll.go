package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"devlog/internal/config"
	"devlog/internal/llm"
	"devlog/internal/modules"
	"devlog/internal/storage"
	"devlog/plugins/summarizer"

	"github.com/urfave/cli/v2"
)

func PollCommand() *cli.Command {
	return &cli.Command{
		Name:      "poll",
		Usage:     "Manually poll a module or plugin for events (developer/debug command)",
		ArgsUsage: "<name>",
		Hidden:    true,
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return fmt.Errorf("component name required")
			}

			name := c.Args().First()

			fmt.Println("Note: 'devlog poll' is a manual testing command and does not affect the daemon's scheduled pollers.")

			if name == "summarizer" {
				return pollSummarizer()
			}

			mod, err := modules.Get(name)
			if err != nil {
				return fmt.Errorf("unknown module or plugin: %s", name)
			}

			pollable, ok := mod.(modules.ModuleWithPoller)
			if !ok {
				return fmt.Errorf("module %s does not support polling", name)
			}

			return pollModule(name, pollable)
		},
	}
}

func pollModule(name string, mod modules.ModuleWithPoller) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if !cfg.IsModuleEnabled(name) {
		return fmt.Errorf("%s module is not enabled (run 'devlog module install %s' first)", name, name)
	}

	modCfg, ok := cfg.GetModuleConfig(name)
	if !ok {
		return fmt.Errorf("%s module config not found", name)
	}

	dataDir, err := config.DataDir()
	if err != nil {
		return fmt.Errorf("get data directory: %w", err)
	}

	fmt.Printf("Polling %s module...\n\n", name)

	poller, err := mod.CreatePoller(modCfg, dataDir)
	if err != nil {
		return fmt.Errorf("create poller: %w", err)
	}

	events, err := poller.Poll(context.Background())
	if err != nil {
		return fmt.Errorf("poll: %w", err)
	}

	if len(events) == 0 {
		fmt.Println("No new events found")
		return nil
	}

	fmt.Printf("Found %d new events:\n\n", len(events))

	dbPath := filepath.Join(dataDir, "events.db")
	store, err := storage.New(dbPath)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer store.Close()

	storedCount := 0
	for i, evt := range events {
		fmt.Printf("[%d] %s - %s/%s\n", i+1, evt.Timestamp, evt.Source, evt.Type)

		if evt.Repo != "" {
			fmt.Printf("    Repo: %s\n", evt.Repo)
		}
		if evt.Branch != "" {
			fmt.Printf("    Branch: %s\n", evt.Branch)
		}

		if summary, ok := evt.Payload["summary"].(string); ok && summary != "" {
			fmt.Printf("    Summary: %s\n", summary)
		}
		if text, ok := evt.Payload["text"].(string); ok && text != "" {
			displayText := text
			if len(displayText) > 100 {
				displayText = displayText[:100] + "..."
			}
			fmt.Printf("    Text: %s\n", displayText)
		}
		if cmd, ok := evt.Payload["command"].(string); ok && cmd != "" {
			displayCmd := cmd
			if len(displayCmd) > 80 {
				displayCmd = displayCmd[:80] + "..."
			}
			fmt.Printf("    Command: %s\n", displayCmd)
		}
		if filePath, ok := evt.Payload["file_path"].(string); ok && filePath != "" {
			fmt.Printf("    File: %s\n", filePath)
		}

		fmt.Println()

		if err := store.InsertEvent(evt); err != nil {
			if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
				fmt.Fprintf(os.Stderr, "Warning: failed to store event: %v\n", err)
			}
		} else {
			storedCount++
		}
	}

	if storedCount > 0 {
		fmt.Printf("✓ Stored %d new event(s) in database\n", storedCount)
	} else {
		fmt.Println("✓ No new events to store (all entries already captured)")
	}
	fmt.Println()
	fmt.Println("Run 'devlog status' to see the events")

	return nil
}

func pollSummarizer() error {
	fmt.Println("Triggering manual summary generation...")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if !cfg.IsPluginEnabled("summarizer") {
		return fmt.Errorf("summarizer plugin is not enabled (run 'devlog plugin install summarizer' first)")
	}

	pluginCfg, ok := cfg.GetPluginConfig("summarizer")
	if !ok {
		return fmt.Errorf("summarizer plugin config not found")
	}

	dataDir, err := config.DataDir()
	if err != nil {
		return fmt.Errorf("get data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "events.db")
	store, err := storage.New(dbPath)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer store.Close()

	intervalMins := 15
	if val, ok := pluginCfg["interval_minutes"]; ok {
		switch v := val.(type) {
		case float64:
			intervalMins = int(v)
		case int:
			intervalMins = v
		}
	}

	contextWindowMins := 60
	if val, ok := pluginCfg["context_window_minutes"]; ok {
		switch v := val.(type) {
		case float64:
			contextWindowMins = int(v)
		case int:
			contextWindowMins = v
		}
	}

	if !cfg.IsPluginEnabled("llm") {
		return fmt.Errorf("llm plugin is not enabled (required by summarizer)")
	}

	llmCfg, ok := cfg.GetPluginConfig("llm")
	if !ok {
		return fmt.Errorf("llm plugin config not found")
	}

	provider := "ollama"
	if p, ok := llmCfg["provider"].(string); ok {
		provider = p
	}

	apiKey := ""
	if k, ok := llmCfg["api_key"].(string); ok {
		apiKey = k
	}

	baseURL := ""
	if u, ok := llmCfg["base_url"].(string); ok {
		baseURL = u
	}

	model := ""
	if m, ok := llmCfg["model"].(string); ok {
		model = m
	}

	llmConfig := llm.Config{
		Provider: llm.ProviderType(provider),
		APIKey:   apiKey,
		BaseURL:  baseURL,
		Model:    model,
	}

	llmClient, err := llm.NewClient(llmConfig)
	if err != nil {
		return fmt.Errorf("create LLM client: %w", err)
	}

	interval := time.Duration(intervalMins) * time.Minute
	contextWindow := time.Duration(contextWindowMins) * time.Minute

	var excludeSources []string
	if val, ok := pluginCfg["exclude_sources"]; ok {
		if arr, ok := val.([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					excludeSources = append(excludeSources, s)
				}
			}
		}
	}

	fmt.Printf("Configuration:\n")
	fmt.Printf("  Provider: %s\n", provider)
	fmt.Printf("  Interval: %d minutes\n", intervalMins)
	fmt.Printf("  Context window: %d minutes\n", contextWindowMins)
	if len(excludeSources) > 0 {
		fmt.Printf("  Excluding sources: %v\n", excludeSources)
	}
	fmt.Println()

	plugin := summarizer.NewForPoll(llmClient, store, interval, contextWindow, excludeSources)

	fmt.Println("Generating summary...")
	if err := plugin.GenerateSummaryNow(context.Background()); err != nil {
		return fmt.Errorf("generate summary: %w", err)
	}

	summariesDir := filepath.Join(dataDir, "summaries")
	filename := fmt.Sprintf("summary_%s.md", time.Now().Format("2006-01-02"))
	path := filepath.Join(summariesDir, filename)

	fmt.Printf("✓ Summary saved to %s\n", path)
	fmt.Println()
	fmt.Println("Run 'devlog status' to see the latest activity")

	return nil
}
