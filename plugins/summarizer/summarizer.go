package summarizer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"devlog/internal/config"
	"devlog/internal/contextkeys"
	"devlog/internal/errors"
	"devlog/internal/events"
	"devlog/internal/install"
	"devlog/internal/llm"
	"devlog/internal/logger"
	"devlog/internal/metrics"
	"devlog/internal/plugins"
	"devlog/internal/storage"
)

type Plugin struct {
	llmClient      llm.Client
	storage        *storage.Storage
	interval       time.Duration
	contextWindow  time.Duration
	excludeSources map[string]bool
	logger         *logger.Logger
}

type Config struct {
	IntervalMinutes      int      `json:"interval_minutes"`
	ContextWindowMinutes int      `json:"context_window_minutes"`
	ExcludeSources       []string `json:"exclude_sources"`
}

func init() {
	plugins.Register(&Plugin{})
}

func (p *Plugin) Name() string {
	return "summarizer"
}

func (p *Plugin) Description() string {
	return "Periodically summarizes dev activity using an LLM"
}

func (p *Plugin) Metadata() plugins.Metadata {
	return plugins.Metadata{
		Name:         "summarizer",
		Description:  "Periodically summarizes dev activity using an LLM",
		Dependencies: []string{"llm"},
	}
}

func (p *Plugin) Install(ctx *install.Context) error {
	ctx.Log("Installing Summarizer plugin")
	ctx.Log("This plugin requires the 'llm' plugin to be enabled")
	ctx.Log("Configure the LLM provider in the 'llm' plugin configuration")
	return nil
}

func (p *Plugin) Uninstall(ctx *install.Context) error {
	ctx.Log("Uninstalling Summarizer plugin")
	return nil
}

func (p *Plugin) DefaultConfig() interface{} {
	return &Config{
		IntervalMinutes:      30,
		ContextWindowMinutes: 60,
		ExcludeSources:       []string{"clipboard"},
	}
}

func (p *Plugin) ValidateConfig(config interface{}) error {
	cfgMap, ok := config.(map[string]interface{})
	if !ok {
		return errors.NewValidation("config", "must be a map")
	}

	var interval float64
	if val, ok := cfgMap["interval_minutes"]; !ok {
		return errors.NewValidation("interval_minutes", "is required")
	} else {
		switch v := val.(type) {
		case float64:
			interval = v
		case int:
			interval = float64(v)
		default:
			return errors.NewValidation("interval_minutes", "must be a number")
		}
	}
	if interval < 1 || interval > 1440 {
		return errors.NewValidation("interval_minutes", "must be between 1 and 1440")
	}

	var contextWindow float64
	if val, ok := cfgMap["context_window_minutes"]; !ok {
		return errors.NewValidation("context_window_minutes", "is required")
	} else {
		switch v := val.(type) {
		case float64:
			contextWindow = v
		case int:
			contextWindow = float64(v)
		default:
			return errors.NewValidation("context_window_minutes", "must be a number")
		}
	}
	if contextWindow < 1 || contextWindow > 1440 {
		return errors.NewValidation("context_window_minutes", "must be between 1 and 1440")
	}

	if contextWindow < interval {
		return errors.NewValidation("context_window_minutes", "must be greater than or equal to interval_minutes")
	}

	return nil
}

func (p *Plugin) InjectServices(services map[string]interface{}) error {
	llmClient, ok := services["llm.client"]
	if !ok {
		return errors.WrapPlugin("summarizer", "inject services", fmt.Errorf("llm.client service not found"))
	}

	client, ok := llmClient.(llm.Client)
	if !ok {
		return errors.WrapPlugin("summarizer", "inject services", fmt.Errorf("llm.client service has wrong type"))
	}

	p.llmClient = client
	return nil
}

func (p *Plugin) Start(ctx context.Context) error {
	cfgMap, ok := ctx.Value(contextkeys.PluginConfig).(map[string]interface{})
	if !ok || cfgMap == nil {
		return errors.WrapPlugin("summarizer", "start", fmt.Errorf("plugin config not found in context"))
	}

	cfg := &Config{}
	cfgBytes, err := json.Marshal(cfgMap)
	if err != nil {
		return errors.WrapPlugin("summarizer", "marshal config", err)
	}
	if err := json.Unmarshal(cfgBytes, cfg); err != nil {
		return errors.WrapPlugin("summarizer", "unmarshal config", err)
	}

	p.interval = time.Duration(cfg.IntervalMinutes) * time.Minute
	p.contextWindow = time.Duration(cfg.ContextWindowMinutes) * time.Minute
	p.excludeSources = make(map[string]bool)
	for _, source := range cfg.ExcludeSources {
		p.excludeSources[source] = true
	}
	p.logger = logger.Default()

	dataDir, err := config.DataDir()
	if err != nil {
		return errors.WrapPlugin("summarizer", "get data dir", err)
	}
	dbPath := filepath.Join(dataDir, "events.db")

	store, err := storage.New(dbPath)
	if err != nil {
		return errors.WrapPlugin("summarizer", "open storage", err)
	}
	p.storage = store

	p.run(ctx)

	return nil
}

func (p *Plugin) calculateNextRunTime() time.Time {
	now := time.Now()
	intervalMins := int(p.interval.Minutes())

	currentMinute := now.Minute()
	minutesToBoundary := intervalMins - (currentMinute % intervalMins)

	if minutesToBoundary == intervalMins {
		minutesToBoundary = 0
	}

	nextBoundary := now.Truncate(time.Minute).Add(time.Duration(minutesToBoundary) * time.Minute)

	if time.Until(nextBoundary) < 5*time.Second {
		nextBoundary = nextBoundary.Add(p.interval)
	}

	return nextBoundary
}

func (p *Plugin) isAtBoundary(t time.Time) bool {
	intervalMins := int(p.interval.Minutes())
	return t.Minute()%intervalMins == 0
}

func (p *Plugin) run(ctx context.Context) {
	nextRun := p.calculateNextRunTime()

	p.logger.Info("summarizer started",
		slog.Duration("interval", p.interval),
		slog.Duration("context_window", p.contextWindow),
		slog.Time("next_run", nextRun))

	for {
		delay := time.Until(nextRun)

		p.logger.Debug("waiting until next boundary",
			slog.Time("next_run", nextRun),
			slog.Duration("delay", delay))

		timer := time.NewTimer(delay)

		select {
		case <-ctx.Done():
			timer.Stop()
			p.logger.Info("summarizer stopped")
			if p.storage != nil {
				p.storage.Close()
			}
			return
		case now := <-timer.C:
			if p.isAtBoundary(now) {
				p.logger.Debug("boundary reached, generating summary")
				if err := p.generateSummary(ctx); err != nil {
					p.logger.Error("failed to generate summary",
						slog.String("error", err.Error()))
				}
			}
			nextRun = p.calculateNextRunTime()
		}
	}
}

func (p *Plugin) generateSummary(ctx context.Context) error {
	timer := metrics.StartPluginTimer("summarizer")
	defer timer.Stop()

	now := time.Now()
	focusEnd := now
	focusStart := now.Add(-p.interval)
	contextStart := focusStart.Add(-p.contextWindow)

	return p.GenerateSummaryForPeriod(ctx, focusStart, focusEnd, contextStart)
}

func (p *Plugin) GenerateSummaryForPeriod(ctx context.Context, focusStart, focusEnd, contextStart time.Time) error {
	contextEvents, err := p.storage.QueryEventsContext(ctx, storage.QueryOptions{
		StartTime: &contextStart,
		EndTime:   &focusEnd,
	})
	if err != nil {
		return fmt.Errorf("list context events: %w", err)
	}

	focusEvents, err := p.storage.QueryEventsContext(ctx, storage.QueryOptions{
		StartTime: &focusStart,
		EndTime:   &focusEnd,
	})
	if err != nil {
		return fmt.Errorf("list focus events: %w", err)
	}

	filteredContextEvents := p.filterEvents(contextEvents)
	filteredFocusEvents := p.filterEvents(focusEvents)

	if len(filteredFocusEvents) == 0 {
		p.logger.Debug("no events in focus window, generating placeholder")
		if err := p.saveSummary("", focusStart, focusEnd, filteredContextEvents, filteredFocusEvents); err != nil {
			return fmt.Errorf("save summary: %w", err)
		}
		return nil
	}

	prompt := buildPrompt(filteredContextEvents, filteredFocusEvents, FormatEvent)

	p.logger.Debug("requesting LLM summary",
		slog.Int("context_events", len(filteredContextEvents)),
		slog.Int("focus_events", len(filteredFocusEvents)))

	summary, err := p.llmClient.Complete(ctx, prompt)
	if err != nil {
		return fmt.Errorf("generate summary: %w", err)
	}

	summary = strings.TrimSpace(summary)
	if summary == "" {
		return fmt.Errorf("empty summary from LLM")
	}

	if err := p.saveSummary(summary, focusStart, focusEnd, filteredContextEvents, filteredFocusEvents); err != nil {
		return fmt.Errorf("save summary: %w", err)
	}

	p.logger.Info("summary generated",
		slog.Int("context_events", len(filteredContextEvents)),
		slog.Int("focus_events", len(filteredFocusEvents)))

	return nil
}

func (p *Plugin) filterEvents(evts []*events.Event) []*events.Event {
	if len(p.excludeSources) == 0 {
		return evts
	}

	filtered := make([]*events.Event, 0, len(evts))
	for _, evt := range evts {
		if !p.excludeSources[evt.Source] {
			filtered = append(filtered, evt)
		}
	}
	return filtered
}

func extractFilesFromCommand(cmd string) []string {
	var files []string

	patterns := []struct {
		regex *regexp.Regexp
		group int
	}{
		{regexp.MustCompile(`(?:vim|vi|nvim|nano|emacs|code|subl)\s+([^\s]+)`), 1},
		{regexp.MustCompile(`(?:cat|less|more|head|tail)\s+([^\s]+)`), 1},
		{regexp.MustCompile(`sed\s+.*?\s+([^\s][^\s]*\.[^\s]+)(?:\s|$)`), 1},
		{regexp.MustCompile(`awk\s+.*?\s+([^\s][^\s]*\.[^\s]+)(?:\s|$)`), 1},
		{regexp.MustCompile(`echo\s+.*?>\s*([^\s]+)`), 1},
		{regexp.MustCompile(`(?:cp|mv)\s+[^\s]+\s+([^\s]+)`), 1},
		{regexp.MustCompile(`(?:touch|rm|chmod|chown)\s+([^\s]+)`), 1},
		{regexp.MustCompile(`git\s+(?:add|rm|mv|checkout)\s+([^\s-][^\s]*)`), 1},
	}

	for _, pattern := range patterns {
		if matches := pattern.regex.FindAllStringSubmatch(cmd, -1); matches != nil {
			for _, match := range matches {
				if len(match) > pattern.group {
					file := match[pattern.group]
					if file != "" && !strings.HasPrefix(file, "-") && !strings.HasPrefix(file, "'") && !strings.HasPrefix(file, "\"") {
						files = append(files, file)
					}
				}
			}
		}
	}

	return files
}

func FormatEvent(evt *events.Event) string {
	line := fmt.Sprintf("\n[%s] %s/%s", evt.Timestamp, evt.Source, evt.Type)

	if evt.Repo != "" {
		line += fmt.Sprintf(" (repo: %s)", evt.Repo)
	}
	if evt.Branch != "" {
		line += fmt.Sprintf(" (branch: %s)", evt.Branch)
	}

	if workdir, ok := evt.Payload["workdir"].(string); ok && workdir != "" {
		line += fmt.Sprintf(" (workdir: %s)", workdir)
	}

	if summary, ok := evt.Payload["summary"].(string); ok && summary != "" {
		if len(summary) > 200 {
			summary = summary[:200] + "..."
		}
		line += fmt.Sprintf(": %s", summary)
	} else if msg, ok := evt.Payload["message"].(string); ok && msg != "" {
		line += fmt.Sprintf(": %s", msg)
	} else if cmd, ok := evt.Payload["command"].(string); ok && cmd != "" {
		line += fmt.Sprintf(": %s", cmd)

		files := extractFilesFromCommand(cmd)
		if len(files) > 0 {
			line += fmt.Sprintf(" [files: %s]", strings.Join(files, ", "))
		}
	} else if text, ok := evt.Payload["text"].(string); ok && text != "" {
		if len(text) > 100 {
			text = text[:100] + "..."
		}
		line += fmt.Sprintf(": %s", text)
	}

	return line
}

func (p *Plugin) buildMarkdownSection(summary string, focusStart, focusEnd time.Time, contextEvents, focusEvents []*events.Event) string {
	var section strings.Builder

	section.WriteString(fmt.Sprintf("## %s - %s\n\n",
		focusStart.Format("15:04"),
		focusEnd.Format("15:04")))

	if len(focusEvents) == 0 {
		section.WriteString("No development activity recorded during this period.\n\n")
	} else {
		section.WriteString(summary)
		section.WriteString("\n\n")
	}

	section.WriteString(p.buildDebugSection(focusStart, focusEnd, contextEvents, focusEvents))

	return section.String()
}

func extractEventContent(evt *events.Event) string {
	if summary, ok := evt.Payload["summary"].(string); ok && summary != "" {
		return summary
	}
	if msg, ok := evt.Payload["message"].(string); ok && msg != "" {
		return msg
	}
	if cmd, ok := evt.Payload["command"].(string); ok && cmd != "" {
		return cmd
	}
	if content, ok := evt.Payload["content"].(string); ok && content != "" {
		return content
	}
	if text, ok := evt.Payload["text"].(string); ok && text != "" {
		return text
	}
	return ""
}

func (p *Plugin) buildDebugSection(focusStart, focusEnd time.Time, contextEvents, focusEvents []*events.Event) string {
	var debug strings.Builder

	contextStart := focusEnd.Add(-p.contextWindow)

	debug.WriteString("<details>\n<summary>Debug Info</summary>\n\n")
	debug.WriteString("```\n")
	debug.WriteString("Time Windows:\n")
	debug.WriteString(fmt.Sprintf("  Context: %s to %s (%s)\n",
		contextStart.Format("15:04:05"),
		focusEnd.Format("15:04:05"),
		p.contextWindow))
	debug.WriteString(fmt.Sprintf("  Focus:   %s to %s (%s)\n",
		focusStart.Format("15:04:05"),
		focusEnd.Format("15:04:05"),
		p.interval))
	debug.WriteString("\nEvent Counts:\n")
	debug.WriteString(fmt.Sprintf("  Context: %d events\n", len(contextEvents)))
	debug.WriteString(fmt.Sprintf("  Focus:   %d events\n", len(focusEvents)))
	debug.WriteString("```\n\n")

	contextBySource := groupEventsBySource(contextEvents)
	focusBySource := groupEventsBySource(focusEvents)

	debug.WriteString("### Context Events (background only)\n\n")
	if len(contextEvents) == 0 {
		debug.WriteString("_No context events_\n\n")
	} else {
		debug.WriteString("```\n")
		for source, events := range contextBySource {
			debug.WriteString(fmt.Sprintf("%s (%d events):\n", source, len(events)))
			for _, evt := range events {
				ts, _ := time.Parse(time.RFC3339, evt.Timestamp)
				debug.WriteString(fmt.Sprintf("  %s [%s] %s/%s",
					ts.Format("15:04:05"),
					evt.ID[:8],
					evt.Source,
					evt.Type))
				if evt.Repo != "" {
					debug.WriteString(fmt.Sprintf(" (%s", evt.Repo))
					if evt.Branch != "" {
						debug.WriteString(fmt.Sprintf(":%s", evt.Branch))
					}
					debug.WriteString(")")
				}
				if content := extractEventContent(evt); content != "" {
					if len(content) > 80 {
						content = content[:80] + "..."
					}
					debug.WriteString(fmt.Sprintf(": %s", content))
				}
				debug.WriteString("\n")
			}
			debug.WriteString("\n")
		}
		debug.WriteString("```\n\n")
	}

	debug.WriteString("### Focus Events (summarized period)\n\n")
	if len(focusEvents) == 0 {
		debug.WriteString("_No focus events_\n\n")
	} else {
		debug.WriteString("```\n")
		for source, events := range focusBySource {
			debug.WriteString(fmt.Sprintf("%s (%d events):\n", source, len(events)))
			for _, evt := range events {
				ts, _ := time.Parse(time.RFC3339, evt.Timestamp)
				debug.WriteString(fmt.Sprintf("  %s [%s] %s/%s",
					ts.Format("15:04:05"),
					evt.ID[:8],
					evt.Source,
					evt.Type))
				if evt.Repo != "" {
					debug.WriteString(fmt.Sprintf(" (%s", evt.Repo))
					if evt.Branch != "" {
						debug.WriteString(fmt.Sprintf(":%s", evt.Branch))
					}
					debug.WriteString(")")
				}
				if content := extractEventContent(evt); content != "" {
					if len(content) > 80 {
						content = content[:80] + "..."
					}
					debug.WriteString(fmt.Sprintf(": %s", content))
				}
				debug.WriteString("\n")
			}
			debug.WriteString("\n")
		}
		debug.WriteString("```\n\n")
	}

	debug.WriteString("</details>\n\n")

	return debug.String()
}

func (p *Plugin) updateOrCreateInactivePeriod(path string, focusStart, focusEnd time.Time) error {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			header := fmt.Sprintf("# Development Summary - %s\n\n", focusStart.Format("January 2, 2006"))
			section := p.buildInactivePeriodSection(focusStart, focusEnd)
			return os.WriteFile(path, []byte(header+section), 0644)
		}
		return fmt.Errorf("read summary file: %w", err)
	}

	inactivePeriodRegex := regexp.MustCompile(`(?m)^## (\d{2}:\d{2}) - (\d{2}:\d{2})\n\nNo development activity recorded during this period\.\n\n`)
	allSectionRegex := regexp.MustCompile(`(?m)^## (\d{2}:\d{2}) - (\d{2}:\d{2})\n\n`)

	inactiveMatches := inactivePeriodRegex.FindAllStringSubmatchIndex(string(content), -1)
	allMatches := allSectionRegex.FindAllStringSubmatchIndex(string(content), -1)

	if len(inactiveMatches) > 0 {
		lastInactiveMatch := inactiveMatches[len(inactiveMatches)-1]
		lastInactiveStart := string(content[lastInactiveMatch[2]:lastInactiveMatch[3]])
		lastInactiveIdx := lastInactiveMatch[0]

		for _, match := range allMatches {
			if match[0] > lastInactiveIdx {
				isInactive := false
				for _, inactiveMatch := range inactiveMatches {
					if match[0] == inactiveMatch[0] {
						isInactive = true
						break
					}
				}
				if !isInactive {
					section := p.buildInactivePeriodSection(focusStart, focusEnd)
					f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
					if err != nil {
						return fmt.Errorf("open summary file: %w", err)
					}
					defer f.Close()
					if _, err := f.WriteString(section); err != nil {
						return fmt.Errorf("write inactive period: %w", err)
					}
					return nil
				}
			}
		}

		updatedSection := fmt.Sprintf("## %s - %s\n\nNo development activity recorded during this period.\n\n",
			lastInactiveStart,
			focusEnd.Format("15:04"))

		newContent := string(content[:lastInactiveMatch[0]]) + updatedSection + string(content[lastInactiveMatch[1]:])
		return os.WriteFile(path, []byte(newContent), 0644)
	}

	section := p.buildInactivePeriodSection(focusStart, focusEnd)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open summary file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(section); err != nil {
		return fmt.Errorf("write inactive period: %w", err)
	}

	return nil
}

func (p *Plugin) buildInactivePeriodSection(focusStart, focusEnd time.Time) string {
	return fmt.Sprintf("## %s - %s\n\nNo development activity recorded during this period.\n\n",
		focusStart.Format("15:04"),
		focusEnd.Format("15:04"))
}

func (p *Plugin) saveSummary(summary string, focusStart, focusEnd time.Time, contextEvents, focusEvents []*events.Event) error {
	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	summariesDir := filepath.Join(dataDir, "summaries")
	if err := os.MkdirAll(summariesDir, 0755); err != nil {
		return fmt.Errorf("create summaries dir: %w", err)
	}

	filename := fmt.Sprintf("summary_%s.md", focusStart.Format("2006-01-02"))
	path := filepath.Join(summariesDir, filename)

	if len(focusEvents) == 0 {
		if err := p.updateOrCreateInactivePeriod(path, focusStart, focusEnd); err != nil {
			return err
		}
	} else {
		section := p.buildMarkdownSection(summary, focusStart, focusEnd, contextEvents, focusEvents)

		if _, err := os.Stat(path); os.IsNotExist(err) {
			header := fmt.Sprintf("# Development Summary - %s\n\n", focusStart.Format("January 2, 2006"))
			section = header + section
		}

		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("open summary file: %w", err)
		}
		defer f.Close()

		if _, err := f.WriteString(section); err != nil {
			return fmt.Errorf("write summary section: %w", err)
		}
	}

	p.logger.Info("summary appended",
		slog.String("file", filename),
		slog.String("period", fmt.Sprintf("%s - %s", focusStart.Format("15:04"), focusEnd.Format("15:04"))),
		slog.Int("events", len(focusEvents)))

	return nil
}

func NewForPoll(llmClient llm.Client, store *storage.Storage, interval, contextWindow time.Duration, excludeSources []string) *Plugin {
	excludeMap := make(map[string]bool)
	for _, source := range excludeSources {
		excludeMap[source] = true
	}
	return &Plugin{
		llmClient:      llmClient,
		storage:        store,
		interval:       interval,
		contextWindow:  contextWindow,
		excludeSources: excludeMap,
		logger:         logger.Default(),
	}
}

func (p *Plugin) GenerateSummaryNow(ctx context.Context) error {
	return p.generateSummary(ctx)
}
