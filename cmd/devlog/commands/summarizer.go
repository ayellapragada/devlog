package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"devlog/internal/config"
	"devlog/internal/llm"
	"devlog/internal/storage"
	"devlog/plugins/summarizer"

	"github.com/urfave/cli/v2"
)

func SummarizerCommand() *cli.Command {
	return &cli.Command{
		Name:  "summarizer",
		Usage: "Use extra functionality to aid the summarizer",
		Subcommands: []*cli.Command{
			{
				Name:      "backfill",
				Usage:     "Backfill summaries for a specific day (defaults to today)",
				ArgsUsage: "[day]",
				Action:    backfillAction,
			},
			{
				Name:   "open",
				Usage:  "Open the latest summary file",
				Action: openAction,
			},
		},
	}
}

func backfillAction(c *cli.Context) error {
	dayStr := "today"
	if c.Args().Present() {
		dayStr = c.Args().First()
	}

	day, err := parseDay(dayStr)
	if err != nil {
		return fmt.Errorf("parse day: %w", err)
	}

	dataDir, err := config.DataDir()
	if err != nil {
		return fmt.Errorf("get data directory: %w", err)
	}

	summariesDir := filepath.Join(dataDir, "summaries")
	filename := fmt.Sprintf("summary_%s.md", day.Format("2006-01-02"))
	summaryPath := filepath.Join(summariesDir, filename)

	if _, err := os.Stat(summaryPath); err == nil {
		fmt.Printf("Deleting existing summary: %s\n", filename)
		if err := os.Remove(summaryPath); err != nil {
			return fmt.Errorf("delete existing summary: %w", err)
		}
	}

	return backfillSummarizer(day, day.AddDate(0, 0, 1), dataDir)
}

func parseDay(dayStr string) (time.Time, error) {
	now := time.Now()

	switch dayStr {
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), nil
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		return time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location()), nil
	}

	t, err := time.Parse("2006-01-02", dayStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid day format: %s (use '2006-01-02', 'today', or 'yesterday')", dayStr)
	}

	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, now.Location()), nil
}

func backfillSummarizer(start, end time.Time, dataDir string) error {
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

	dbPath := filepath.Join(dataDir, "events.db")
	store, err := storage.New(dbPath)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer store.Close()

	intervalMins := 30
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

	fmt.Printf("Backfilling summaries for %s:\n", start.Format("2006-01-02"))
	fmt.Printf("  Interval: %d minutes\n", intervalMins)
	fmt.Printf("  Context window: %d minutes\n", contextWindowMins)
	fmt.Printf("  Provider: %s\n", provider)
	if len(excludeSources) > 0 {
		fmt.Printf("  Excluding sources: %v\n", excludeSources)
	}
	fmt.Println()

	current := start
	count := 0

	for current.Before(end) {
		focusEnd := current.Add(interval)
		if focusEnd.After(end) {
			focusEnd = end
		}

		fmt.Printf("[%s - %s] ", current.Format("15:04"), focusEnd.Format("15:04"))

		plugin := summarizer.NewForPoll(llmClient, store, interval, contextWindow, excludeSources)
		ctx := context.Background()
		contextStart := current.Add(-contextWindow)

		if err := plugin.GenerateSummaryForPeriod(ctx, current, focusEnd, contextStart); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			return err
		}

		fmt.Printf("✓\n")
		count++
		current = focusEnd
	}

	fmt.Printf("\n✓ Generated %d summaries for %s\n", count, start.Format("2006-01-02"))
	return nil
}

func openAction(c *cli.Context) error {
	dataDir, err := config.DataDir()
	if err != nil {
		return fmt.Errorf("get data directory: %w", err)
	}

	summariesDir := filepath.Join(dataDir, "summaries")
	if _, err := os.Stat(summariesDir); os.IsNotExist(err) {
		return fmt.Errorf("no summaries found: %s does not exist", summariesDir)
	}

	entries, err := os.ReadDir(summariesDir)
	if err != nil {
		return fmt.Errorf("read summaries directory: %w", err)
	}

	var summaryFiles []os.FileInfo
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".md" {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			summaryFiles = append(summaryFiles, info)
		}
	}

	if len(summaryFiles) == 0 {
		return fmt.Errorf("no summary files found in %s", summariesDir)
	}

	sort.Slice(summaryFiles, func(i, j int) bool {
		return summaryFiles[i].ModTime().After(summaryFiles[j].ModTime())
	})

	latestFile := filepath.Join(summariesDir, summaryFiles[0].Name())
	absPath, err := filepath.Abs(latestFile)
	if err != nil {
		return fmt.Errorf("get absolute path: %w", err)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		browsers := []string{
			"Google Chrome",
			"Brave Browser",
			"Firefox",
			"Safari",
			"Microsoft Edge",
			"Arc",
			"Chromium",
		}

		var openErr error
		for _, browser := range browsers {
			cmd = exec.Command("open", "-a", browser, absPath)
			if err := cmd.Start(); err == nil {
				fmt.Printf("Opening %s in %s...\n", summaryFiles[0].Name(), browser)
				return nil
			}
			openErr = err
		}
		return fmt.Errorf("failed to open in any browser: %w", openErr)

	case "linux":
		browsers := []string{"google-chrome", "chrome", "chromium", "firefox", "brave-browser"}
		var openErr error
		for _, browser := range browsers {
			cmd = exec.Command(browser, absPath)
			if err := cmd.Start(); err == nil {
				fmt.Printf("Opening %s in browser...\n", summaryFiles[0].Name())
				return nil
			}
			openErr = err
		}
		return fmt.Errorf("failed to open in any browser: %w", openErr)

	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "chrome", absPath)
		if err := cmd.Start(); err != nil {
			cmd = exec.Command("cmd", "/c", "start", "firefox", absPath)
			if err := cmd.Start(); err != nil {
				return fmt.Errorf("failed to open browser: %w", err)
			}
		}
		fmt.Printf("Opening %s in browser...\n", summaryFiles[0].Name())
		return nil

	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
