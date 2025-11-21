package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"devlog/internal/config"
	"devlog/internal/llm"
	"devlog/internal/storage"
	"devlog/plugins/summarizer"

	"github.com/urfave/cli/v2"
)

func SummarizerCommand() *cli.Command {
	return &cli.Command{
		Name:   "summarizer",
		Usage:  "Manage summarizer plugin",
		Hidden: true,
		Subcommands: []*cli.Command{
			{
				Name:      "backfill",
				Usage:     "Backfill summaries for a specific time range",
				ArgsUsage: " ",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "start",
						Usage: "Start time (format: 2006-01-02 15:04 or 15:04 for today)",
						Value: "00:00",
					},
					&cli.StringFlag{
						Name:  "end",
						Usage: "End time (format: 2006-01-02 15:04 or 15:04 for today, or 'now')",
						Value: "now",
					},
				},
				Action: backfillAction,
			},
		},
	}
}

func backfillAction(c *cli.Context) error {
	startStr := c.String("start")
	endStr := c.String("end")

	start, err := parseTimeForBackfill(startStr)
	if err != nil {
		return fmt.Errorf("parse start time: %w", err)
	}

	end, err := parseTimeForBackfill(endStr)
	if err != nil {
		return fmt.Errorf("parse end time: %w", err)
	}

	if end.Before(start) {
		return fmt.Errorf("end time must be after start time")
	}

	return backfillSummarizer(start, end)
}

func parseTimeForBackfill(timeStr string) (time.Time, error) {
	if timeStr == "now" {
		return time.Now(), nil
	}

	formats := []string{
		"2006-01-02 15:04",
		"15:04",
	}

	for _, format := range formats {
		t, err := time.Parse(format, timeStr)
		if err == nil {
			if format == "15:04" {
				now := time.Now()
				t = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
			}
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid time format: %s (use '2006-01-02 15:04' or '15:04' or 'now')", timeStr)
}

func backfillSummarizer(start, end time.Time) error {
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

	fmt.Printf("Backfilling summaries:\n")
	fmt.Printf("  Time range: %s to %s\n", start.Format("2006-01-02 15:04"), end.Format("2006-01-02 15:04"))
	fmt.Printf("  Interval: %d minutes\n", intervalMins)
	fmt.Printf("  Context window: %d minutes\n", contextWindowMins)
	fmt.Printf("  Provider: %s\n", provider)
	if len(excludeSources) > 0 {
		fmt.Printf("  Excluding sources: %v\n", excludeSources)
	}
	fmt.Println()

	current := start
	count := 0
	skipped := 0

	for current.Before(end) {
		focusEnd := current.Add(interval)
		if focusEnd.After(end) {
			focusEnd = end
		}

		fmt.Printf("[%s - %s] ", current.Format("15:04"), focusEnd.Format("15:04"))

		plugin := summarizer.NewForPoll(llmClient, store, interval, contextWindow, excludeSources)

		if err := generateSummaryForBackfill(plugin, store, current, focusEnd, contextWindow); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			return err
		}

		fmt.Printf("✓\n")
		count++
		current = focusEnd
	}

	fmt.Println()
	if skipped > 0 {
		fmt.Printf("✓ Generated %d summaries (%d periods had no events)\n", count, skipped)
	} else {
		fmt.Printf("✓ Generated %d summaries\n", count)
	}

	summariesDir := filepath.Join(dataDir, "summaries")
	filename := fmt.Sprintf("summary_%s.md", start.Format("2006-01-02"))
	path := filepath.Join(summariesDir, filename)
	fmt.Printf("✓ Summaries saved to %s\n", path)

	return nil
}

func generateSummaryForBackfill(plugin *summarizer.Plugin, store *storage.Storage, focusStart, focusEnd time.Time, contextWindow time.Duration) error {
	ctx := context.Background()
	contextStart := focusStart.Add(-contextWindow)

	contextEvents, err := store.QueryEventsContext(ctx, storage.QueryOptions{
		StartTime: &contextStart,
		EndTime:   &focusEnd,
	})
	if err != nil {
		return fmt.Errorf("list context events: %w", err)
	}

	focusEvents, err := store.QueryEventsContext(ctx, storage.QueryOptions{
		StartTime: &focusStart,
		EndTime:   &focusEnd,
	})
	if err != nil {
		return fmt.Errorf("list focus events: %w", err)
	}

	filteredContextEvents := plugin.FilterEvents(contextEvents)
	filteredFocusEvents := plugin.FilterEvents(focusEvents)

	var summary string
	if len(filteredFocusEvents) > 0 {
		prompt := plugin.BuildPrompt(filteredContextEvents, filteredFocusEvents)

		llmClient := plugin.GetLLMClient()
		summary, err = llmClient.Complete(ctx, prompt)
		if err != nil {
			return fmt.Errorf("generate summary: %w", err)
		}
	}

	if err := plugin.SaveSummaryExported(summary, focusStart, focusEnd, filteredContextEvents, filteredFocusEvents); err != nil {
		return fmt.Errorf("save summary: %w", err)
	}

	return nil
}
