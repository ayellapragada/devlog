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
	"devlog/internal/events"
	"devlog/internal/install"
	"devlog/internal/llm"
	"devlog/internal/logger"
	"devlog/internal/metrics"
	"devlog/internal/plugins"
	"devlog/internal/storage"
)

type Plugin struct {
	llmClient     llm.Client
	storage       *storage.Storage
	interval      time.Duration
	contextWindow time.Duration
	logger        *logger.Logger
}

type Config struct {
	Provider             string `json:"provider"`
	APIKey               string `json:"api_key"`
	BaseURL              string `json:"base_url,omitempty"`
	Model                string `json:"model,omitempty"`
	IntervalMinutes      int    `json:"interval_minutes"`
	ContextWindowMinutes int    `json:"context_window_minutes"`
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

func (p *Plugin) Install(ctx *install.Context) error {
	ctx.Log("Installing Summarizer plugin")
	ctx.Log("This plugin requires an LLM provider (ollama, anthropic, etc.)")
	ctx.Log("Configure your provider and API settings in the plugin configuration")
	return nil
}

func (p *Plugin) Uninstall(ctx *install.Context) error {
	ctx.Log("Uninstalling Summarizer plugin")
	return nil
}

func (p *Plugin) DefaultConfig() interface{} {
	return &Config{
		Provider:             "ollama",
		BaseURL:              "http://localhost:11434",
		IntervalMinutes:      15,
		ContextWindowMinutes: 60,
	}
}

func (p *Plugin) ValidateConfig(config interface{}) error {
	cfgMap, ok := config.(map[string]interface{})
	if !ok {
		return fmt.Errorf("config must be a map")
	}

	provider, ok := cfgMap["provider"].(string)
	if !ok || provider == "" {
		return fmt.Errorf("provider is required")
	}

	if provider != "ollama" && provider != "anthropic" {
		return fmt.Errorf("provider must be 'ollama' or 'anthropic'")
	}

	if provider == "anthropic" {
		apiKey, ok := cfgMap["api_key"].(string)
		if !ok || apiKey == "" {
			return fmt.Errorf("api_key is required for anthropic provider")
		}
	}

	if provider == "ollama" {
		baseURL, ok := cfgMap["base_url"].(string)
		if !ok || baseURL == "" {
			return fmt.Errorf("base_url is required for ollama provider")
		}
	}

	var interval float64
	if val, ok := cfgMap["interval_minutes"]; !ok {
		return fmt.Errorf("interval_minutes is required")
	} else {
		switch v := val.(type) {
		case float64:
			interval = v
		case int:
			interval = float64(v)
		default:
			return fmt.Errorf("interval_minutes must be a number")
		}
	}
	if interval < 1 || interval > 1440 {
		return fmt.Errorf("interval_minutes must be between 1 and 1440")
	}

	var contextWindow float64
	if val, ok := cfgMap["context_window_minutes"]; !ok {
		return fmt.Errorf("context_window_minutes is required")
	} else {
		switch v := val.(type) {
		case float64:
			contextWindow = v
		case int:
			contextWindow = float64(v)
		default:
			return fmt.Errorf("context_window_minutes must be a number")
		}
	}
	if contextWindow < 1 || contextWindow > 1440 {
		return fmt.Errorf("context_window_minutes must be between 1 and 1440")
	}

	if contextWindow < interval {
		return fmt.Errorf("context_window_minutes must be greater than or equal to interval_minutes")
	}

	return nil
}

func (p *Plugin) Start(ctx context.Context) error {
	cfgMap, ok := ctx.Value(contextkeys.PluginConfig).(map[string]interface{})
	if !ok || cfgMap == nil {
		return fmt.Errorf("plugin config not found in context")
	}

	cfg := &Config{}
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

	llmCfg := llm.Config{
		Provider: llm.ProviderType(cfg.Provider),
		APIKey:   cfg.APIKey,
		BaseURL:  cfg.BaseURL,
		Model:    cfg.Model,
	}

	llmClient, err := llm.NewClient(llmCfg)
	if err != nil {
		return fmt.Errorf("create LLM client: %w", err)
	}
	p.llmClient = llmClient

	p.interval = time.Duration(cfg.IntervalMinutes) * time.Minute
	p.contextWindow = time.Duration(cfg.ContextWindowMinutes) * time.Minute
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
	contextStart := now.Add(-p.contextWindow)

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

	if len(focusEvents) == 0 {
		p.logger.Debug("no events in focus window, generating placeholder")
		if err := p.saveSummary("", focusStart, focusEnd, focusEvents); err != nil {
			return fmt.Errorf("save summary: %w", err)
		}
		return nil
	}

	prompt := p.buildPrompt(contextEvents, focusEvents)

	p.logger.Debug("requesting LLM summary",
		slog.Int("context_events", len(contextEvents)),
		slog.Int("focus_events", len(focusEvents)))

	summary, err := p.llmClient.Complete(ctx, prompt)
	if err != nil {
		return fmt.Errorf("generate summary: %w", err)
	}

	summary = strings.TrimSpace(summary)
	if summary == "" {
		return fmt.Errorf("empty summary from LLM")
	}

	if err := p.saveSummary(summary, focusStart, focusEnd, focusEvents); err != nil {
		return fmt.Errorf("save summary: %w", err)
	}

	p.logger.Info("summary generated",
		slog.Int("context_events", len(contextEvents)),
		slog.Int("focus_events", len(focusEvents)))

	return nil
}

func (p *Plugin) buildPrompt(contextEvents, focusEvents []*events.Event) string {
	return buildPrompt(contextEvents, focusEvents, p.formatEvent)
}

func buildPrompt(contextEvents, focusEvents []*events.Event, formatter func(*events.Event) string) string {
	contextBySource := groupEventsBySource(contextEvents)
	focusBySource := groupEventsBySource(focusEvents)

	prompt := `You are summarizing a development session. You will be given two sets of events:

1. CONTEXT EVENTS: A longer time period showing what happened before the focus period
2. FOCUS EVENTS: The shorter time period that you need to summarize

Events are organized by source:
- CRITICAL: Claude Code conversations, major project work
- HIGH: GitHub activity, significant commits
- MEDIUM: Git operations, kubectl commands
- LOW: Shell commands, clipboard activity, background activity

CONTEXT EVENTS (for background understanding only):
` + formattedBySource(contextBySource, formatter) + `

FOCUS EVENTS (summarize these):
` + formattedBySource(focusBySource, formatter) + `

===== INSTRUCTIONS =====

Generate a two-part summary:

PART 1 - Context line (max 80 characters):
State the project/repository and branch being worked on. Use format: "Working on: <repo> (<branch>)"

PART 2 - Activity summary (Between 2 and 4 bullet points):
Each bullet must be a complete sentence in past tense describing what was accomplished.

CONSOLIDATION RULES:
- If >3 similar commands/operations: "Ran multiple <type> checks" or "Made several <component> changes"
- If repetitive debugging: "Debugged <issue>" not a list of each debug command
- If exploring/researching: "Investigated <topic> in <files>" not each individual grep/read

PRIORITIZATION (in order of importance):
1. CRITICAL events: Major design decisions, architectural discussions, feature implementations
2. HIGH events: Code commits, PR creation, significant git operations
3. MEDIUM events: Only include if they provide essential context to CRITICAL/HIGH events
4. LOW events: Omit unless they reveal a clear pattern (e.g., "Monitored Kubernetes cluster health")

WRITING RULES:
✓ Use past tense: "Implemented", "Fixed", "Refactored", "Discussed"
✓ Be specific: Include file names, component names, function names when relevant
✓ Be direct: Start bullets with action verbs
✓ Be dense: Pack multiple related actions into one bullet

✗ Never use: "the user", "they", "I", "we", "appears", "seems", "likely", "probably"
✗ No meta-commentary: "Focused on", "Worked on", "Continued to", "Spent time"
✗ No vague actions: "Made changes", "Updated files", "Ran commands"
✗ No question marks or uncertainty

EXAMPLES:

BAD:
Worked on the devlog files

- The user worked on improving the summarizer plugin
- They ran multiple git status commands to check the repository state
- Made changes to some configuration files

BAD:
Working in wisprflow project

- Discussed whether to add a priority field to events or create a new event type
- Considered database migration implications
- Looked at event handling code

GOOD:
Working on: devlog (main)

- Refactored summarizer plugin prompt to enforce information density and consolidation rules in plugins/summarizer/summarizer.go
- Discussed priority system implementation for event types with focus on distinguishing high-value development activities
- Debugged shell integration issues related to event capture timing

GOOD:
Working on: devlog (feature/events)

- Evaluated priority field addition versus new event type creation for handling urgent notifications
- Analyzed database schema migration requirements and backwards compatibility concerns in internal/events/schema.go
- Reviewed event processing pipeline to determine type-checking approach versus database queries

===== OUTPUT FORMAT =====

<One line context, max 80 chars>

- <Bullet point 1: most important activity>
- <Bullet point 2: second most important activity>
- <Bullet point 3: third most important activity IF NEEDED>
- <Bullet point 4: fourth most important activity IF NEEDED>

Generate the summary now, following ALL rules above:`

	return prompt
}

func groupEventsBySource(evts []*events.Event) map[string][]*events.Event {
	grouped := make(map[string][]*events.Event)
	for _, evt := range evts {
		grouped[evt.Source] = append(grouped[evt.Source], evt)
	}
	return grouped
}

func formattedBySource(eventsBySource map[string][]*events.Event, formatter func(*events.Event) string) string {
	var sb strings.Builder

	sources := []struct {
		name  string
		label string
	}{
		{"claude", "CRITICAL"},
		{"github", "HIGH"},
		{"git", "MEDIUM"},
		{"kubectl", "MEDIUM"},
		{"shell", "LOW"},
		{"clipboard", "LOW"},
		{"tmux", "LOW"},
		{"wisprflow", "LOW"},
		{"manual", "MEDIUM"},
	}

	for _, s := range sources {
		evts := eventsBySource[s.name]
		if len(evts) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("\n=== %s: %s (%d events) ===\n", s.label, s.name, len(evts)))
		for _, evt := range evts {
			sb.WriteString(formatter(evt) + "\n")
		}
	}

	return sb.String()
}

func (p *Plugin) formatEvent(evt *events.Event) string {
	return FormatEvent(evt)
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

func (p *Plugin) buildMarkdownSection(summary string, focusStart, focusEnd time.Time, focusEvents []*events.Event) string {
	var section strings.Builder

	section.WriteString(fmt.Sprintf("## %s - %s\n\n",
		focusStart.Format("15:04"),
		focusEnd.Format("15:04")))

	if len(focusEvents) == 0 {
		section.WriteString("_No development activity recorded during this period._\n\n")
	} else {
		section.WriteString(summary)
		section.WriteString("\n\n")
	}

	return section.String()
}

func (p *Plugin) saveSummary(summary string, focusStart, focusEnd time.Time, focusEvents []*events.Event) error {
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

	section := p.buildMarkdownSection(summary, focusStart, focusEnd, focusEvents)

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

	p.logger.Info("summary appended",
		slog.String("file", filename),
		slog.String("period", fmt.Sprintf("%s - %s", focusStart.Format("15:04"), focusEnd.Format("15:04"))),
		slog.Int("events", len(focusEvents)))

	return nil
}

func NewForPoll(llmClient llm.Client, store *storage.Storage, interval, contextWindow time.Duration) *Plugin {
	return &Plugin{
		llmClient:     llmClient,
		storage:       store,
		interval:      interval,
		contextWindow: contextWindow,
		logger:        logger.Default(),
	}
}

func (p *Plugin) GenerateSummaryNow(ctx context.Context) error {
	return p.generateSummary(ctx)
}
