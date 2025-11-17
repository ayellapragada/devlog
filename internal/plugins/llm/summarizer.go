package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"devlog/internal/config"
	"devlog/internal/events"
	"devlog/internal/logger"
	"devlog/internal/plugins"
	"devlog/internal/storage"
)

type Provider interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

type SummarizerPlugin struct {
	provider       Provider
	storage        *storage.Storage
	interval       time.Duration
	eventThreshold int
	lastEventCount int
	logger         *logger.Logger
}

type SummarizerConfig struct {
	Provider        string `json:"provider"`
	APIKey          string `json:"api_key"`
	BaseURL         string `json:"base_url,omitempty"`
	Model           string `json:"model,omitempty"`
	IntervalMinutes int    `json:"interval_minutes"`
	EventLimit      int    `json:"event_limit,omitempty"`
}

func init() {
	plugins.Register(&SummarizerPlugin{})
}

func (p *SummarizerPlugin) Name() string {
	return "llm_summarizer"
}

func (p *SummarizerPlugin) Description() string {
	return "Periodically summarizes dev activity using an LLM"
}

func (p *SummarizerPlugin) Install(ctx *plugins.InstallContext) error {
	ctx.Log("Installing LLM Summarizer plugin")
	ctx.Log("This plugin requires an LLM provider (ollama, anthropic, etc.)")
	ctx.Log("Configure your provider and API settings in the plugin configuration")
	return nil
}

func (p *SummarizerPlugin) Uninstall(ctx *plugins.InstallContext) error {
	ctx.Log("Uninstalling LLM Summarizer plugin")
	return nil
}

func (p *SummarizerPlugin) DefaultConfig() interface{} {
	return &SummarizerConfig{
		Provider:        "ollama",
		IntervalMinutes: 60,
		EventLimit:      100,
	}
}

func (p *SummarizerPlugin) ValidateConfig(config interface{}) error {
	cfg, ok := config.(*SummarizerConfig)
	if !ok {
		return fmt.Errorf("invalid config type")
	}

	if cfg.Provider != "ollama" && cfg.APIKey == "" {
		return fmt.Errorf("api_key is required for non-ollama providers")
	}

	if cfg.IntervalMinutes <= 0 {
		return fmt.Errorf("interval_minutes must be positive")
	}

	if cfg.Provider == "" {
		return fmt.Errorf("provider is required")
	}

	return nil
}

func (p *SummarizerPlugin) Start(ctx context.Context) error {
	cfgMap := ctx.Value("config").(map[string]interface{})

	cfg := &SummarizerConfig{}
	cfgBytes, err := json.Marshal(cfgMap)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := json.Unmarshal(cfgBytes, cfg); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}

	if cfg.Provider == "" {
		cfg.Provider = "ollama"
	}

	if cfg.Provider != "ollama" && cfg.APIKey == "" {
		return fmt.Errorf("api_key is required for provider: %s", cfg.Provider)
	}

	if cfg.IntervalMinutes <= 0 {
		return fmt.Errorf("interval_minutes must be positive")
	}

	switch cfg.Provider {
	case "ollama":
		p.provider = NewOllamaProvider(cfg.BaseURL, cfg.Model)
	case "anthropic":
		p.provider = NewAnthropicProvider(cfg.APIKey, cfg.Model)
	default:
		return fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}

	p.interval = time.Duration(cfg.IntervalMinutes) * time.Minute
	p.eventThreshold = cfg.EventLimit
	if p.eventThreshold == 0 {
		p.eventThreshold = 100
	}
	p.logger = logger.Default()

	dataDir, err := config.DataDir()
	if err != nil {
		return fmt.Errorf("get data dir: %w", err)
	}
	dbPath := filepath.Join(dataDir, "events.db")

	store, err := storage.New(dbPath)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	p.storage = store

	allEvents, err := p.storage.ListEventsContext(ctx, 1, "")
	if err == nil && len(allEvents) > 0 {
		p.lastEventCount = len(allEvents)
	}

	go p.run(ctx)

	return nil
}

func (p *SummarizerPlugin) run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	eventCheckTicker := time.NewTicker(10 * time.Second)
	defer eventCheckTicker.Stop()

	p.logger.Info("LLM summarizer started",
		slog.Duration("interval", p.interval),
		slog.Int("event_threshold", p.eventThreshold))

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("LLM summarizer stopped")
			if p.storage != nil {
				p.storage.Close()
			}
			return
		case <-ticker.C:
			p.logger.Debug("time interval reached, generating summary")
			if err := p.generateSummary(ctx); err != nil {
				p.logger.Error("failed to generate summary",
					slog.String("error", err.Error()))
			}
		case <-eventCheckTicker.C:
			currentCount, err := p.getCurrentEventCount(ctx)
			if err != nil {
				continue
			}

			newEvents := currentCount - p.lastEventCount
			if newEvents >= p.eventThreshold {
				p.logger.Debug("event threshold reached",
					slog.Int("new_events", newEvents),
					slog.Int("threshold", p.eventThreshold))
				if err := p.generateSummary(ctx); err != nil {
					p.logger.Error("failed to generate summary",
						slog.String("error", err.Error()))
				}
			}
		}
	}
}

func (p *SummarizerPlugin) getCurrentEventCount(ctx context.Context) (int, error) {
	rows, err := p.storage.ListEventsContext(ctx, 0, "")
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}

func (p *SummarizerPlugin) generateSummary(ctx context.Context) error {
	limit := p.eventThreshold
	if limit == 0 {
		limit = 100
	}

	allEvents, err := p.storage.ListEventsContext(ctx, limit, "")
	if err != nil {
		return fmt.Errorf("list events: %w", err)
	}

	if len(allEvents) == 0 {
		p.logger.Debug("no events to summarize")
		return nil
	}

	prompt := p.buildPrompt(allEvents)

	p.logger.Debug("requesting LLM summary",
		slog.Int("event_count", len(allEvents)))

	summary, err := p.provider.Complete(ctx, prompt)
	if err != nil {
		return fmt.Errorf("generate summary: %w", err)
	}

	if err := p.saveSummary(summary); err != nil {
		return fmt.Errorf("save summary: %w", err)
	}

	currentCount, err := p.getCurrentEventCount(ctx)
	if err == nil {
		p.lastEventCount = currentCount
	}

	p.logger.Info("summary generated",
		slog.Int("events_processed", len(allEvents)))

	return nil
}

func (p *SummarizerPlugin) buildPrompt(evts []*events.Event) string {
	prompt := "Summarize the following development activity events. Focus on what the developer was working on, key actions taken, and context switches.\n\n"
	prompt += "Events (most recent first):\n"

	for i, evt := range evts {
		if i >= 50 {
			break
		}

		prompt += fmt.Sprintf("\n[%s] %s/%s", evt.Timestamp, evt.Source, evt.Type)

		if evt.Repo != "" {
			prompt += fmt.Sprintf(" (repo: %s)", evt.Repo)
		}
		if evt.Branch != "" {
			prompt += fmt.Sprintf(" (branch: %s)", evt.Branch)
		}

		if msg, ok := evt.Payload["message"].(string); ok && msg != "" {
			prompt += fmt.Sprintf(": %s", msg)
		} else if cmd, ok := evt.Payload["command"].(string); ok && cmd != "" {
			prompt += fmt.Sprintf(": %s", cmd)
		} else if text, ok := evt.Payload["text"].(string); ok && text != "" {
			if len(text) > 100 {
				text = text[:100] + "..."
			}
			prompt += fmt.Sprintf(": %s", text)
		}
	}

	prompt += "\n\nProvide a concise summary (2-3 paragraphs) of the development activity."

	return prompt
}

func (p *SummarizerPlugin) saveSummary(summary string) error {
	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	summariesDir := filepath.Join(dataDir, "summaries")
	if err := os.MkdirAll(summariesDir, 0755); err != nil {
		return fmt.Errorf("create summaries dir: %w", err)
	}

	filename := fmt.Sprintf("summary_%s.txt", time.Now().Format("2006-01-02_15-04-05"))
	path := filepath.Join(summariesDir, filename)

	content := fmt.Sprintf("Generated at: %s\n\n%s\n", time.Now().Format(time.RFC3339), summary)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}

	p.logger.Info("summary saved", slog.String("path", path))

	return nil
}
