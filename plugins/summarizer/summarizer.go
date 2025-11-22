package summarizer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

	filteredContextEvents := p.filterEvents(contextEvents)
	filteredFocusEvents := p.filterEvents(focusEvents)

	if len(filteredFocusEvents) == 0 {
		p.logger.Debug("no events in focus window, generating placeholder")
		if err := p.saveSummary("", focusStart, focusEnd, filteredContextEvents, filteredFocusEvents); err != nil {
			return fmt.Errorf("save summary: %w", err)
		}
		return nil
	}

	prompt := p.buildPrompt(filteredContextEvents, filteredFocusEvents)

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

type repoActivity struct {
	Repo         string
	Branch       string
	EventCount   int
	CriticalHigh int
	MediumLow    int
}

func extractRepoActivity(evts []*events.Event) []repoActivity {
	type key struct {
		repo   string
		branch string
	}

	activityMap := make(map[key]*repoActivity)

	sourcePriority := map[string]int{
		"claude":    3,
		"github":    2,
		"git":       1,
		"kubectl":   1,
		"shell":     0,
		"clipboard": 0,
	}

	for _, evt := range evts {
		if evt.Repo == "" {
			continue
		}

		k := key{repo: evt.Repo, branch: evt.Branch}
		if activityMap[k] == nil {
			activityMap[k] = &repoActivity{
				Repo:   evt.Repo,
				Branch: evt.Branch,
			}
		}

		activityMap[k].EventCount++

		priority := sourcePriority[evt.Source]
		if priority >= 2 {
			activityMap[k].CriticalHigh++
		} else {
			activityMap[k].MediumLow++
		}
	}

	activities := make([]repoActivity, 0, len(activityMap))
	for _, activity := range activityMap {
		activities = append(activities, *activity)
	}

	sort.Slice(activities, func(i, j int) bool {
		if activities[i].CriticalHigh != activities[j].CriticalHigh {
			return activities[i].CriticalHigh > activities[j].CriticalHigh
		}
		if activities[i].MediumLow != activities[j].MediumLow {
			return activities[i].MediumLow > activities[j].MediumLow
		}
		return activities[i].EventCount > activities[j].EventCount
	})

	return activities
}

func (p *Plugin) buildPrompt(contextEvents, focusEvents []*events.Event) string {
	return buildPrompt(contextEvents, focusEvents, p.formatEvent)
}

func buildPrompt(contextEvents, focusEvents []*events.Event, formatter func(*events.Event) string) string {
	contextBySource := groupEventsBySource(contextEvents)
	focusBySource := groupEventsBySource(focusEvents)

	repoActivities := extractRepoActivity(focusEvents)
	repoSection := ""
	if len(repoActivities) > 0 {
		repoSection = "\nACTIVE REPOSITORIES IN FOCUS PERIOD:\n"
		for _, activity := range repoActivities {
			branchInfo := ""
			if activity.Branch != "" {
				branchInfo = fmt.Sprintf(" (%s)", activity.Branch)
			}
			repoSection += fmt.Sprintf("- %s%s: %d events (%d CRITICAL/HIGH, %d MEDIUM/LOW)\n",
				activity.Repo, branchInfo, activity.EventCount, activity.CriticalHigh, activity.MediumLow)
		}
		repoSection += "\n"
	}

	prompt := `You are generating a factual development summary. This is a deterministic
transformation of the provided events, not a creative task. You must ONLY use
information explicitly present in the events. Never guess, infer intent, or
invent missing details.

You will be given two sets of events:

1. CONTEXT EVENTS — older events for background reference only
2. FOCUS EVENTS — the period that MUST be summarized

Events are grouped by source category:
- CRITICAL: Claude Code conversations, major architectural work
- HIGH: GitHub commits, PR activity
- MEDIUM: git commands, kubectl operations
- LOW: shell commands, clipboard activity, misc background
` + repoSection + `
CONTEXT EVENTS (read for background only; DO NOT summarize these):
` + formattedBySource(contextBySource, formatter) + `

FOCUS EVENTS (summarize ONLY these):
` + formattedBySource(focusBySource, formatter) + `

==================== SUMMARY REQUIREMENTS ====================

Your output has exactly two parts:

----------------------------------------------------------------
PART 1 — CONTEXT LINE (one line, max 80 chars)

Format:
Single repo: "Working on: <repo> (<branch>)"
Multiple repos (2-3): "Working on: <repo1> (<branch1>), <repo2> (<branch2>)"
Many repos (4+): "Working on: <repo1> (<branch1>) + N other repos"

Rules:
- Use the ACTIVE REPOSITORIES section above for repo/branch information
- List repos in priority order (already sorted by CRITICAL/HIGH activity)
- If no repo/branch: use "Working on: <primary-topic>"
- Never use asterisks or markdown formatting in the context line
- Keep concise: if listing multiple repos would exceed 80 chars, use "+ N other repos" format
----------------------------------------------------------------

PART 2 — ACTIVITY SUMMARY (2–4 bullet points)

Each bullet MUST:
- Be one complete sentence in past tense
- Start with a strong action verb (not "Implemented clipboard operations" but specific action)
- Include technical specifics: file paths, function names, tool names, error messages
- Consolidate repetitive actions into patterns
- Focus on what was accomplished, not what was attempted

==================== SPECIFICITY GUIDELINES ====================

LEVEL OF DETAIL (aim for the middle):

TOO VAGUE ❌:
- "Implemented clipboard copy operations throughout the session"
- "Discussed and planned model testing for summarizer plugin"
- "Ran multiple terraform plans to manage AWS infrastructure"

TOO DETAILED ❌:
- "Executed terraform plan at 11:41:03, 11:41:19, 11:41:45, and 11:41:58"
- "Ran ./scripts/benchmark_summarizer.sh at 2025-11-20 04:28:39 and 13:47:21"
- "Copied various output logs and configurations related to script testing"

JUST RIGHT ✅:
- "Created benchmark script for testing LLM models on summarizer prompt variants"
- "Debugged terraform lock issue using force-unlock, then validated infrastructure plan"
- "Evaluated qwen2.5:14b and llama3.1:8b for summarization quality and speed"

==================== CONSOLIDATION RULES ====================

You MUST consolidate repetitive or similar events:
- If >3 related operations → describe the goal, not each operation
- Repetitive debugging → "Debugged <specific-issue>" with outcome if known
- Multiple commands for same goal → one bullet describing the objective
- Clipboard/shell spam → OMIT unless it reveals important pattern

EXAMPLES:
- NOT: "Ran benchmark script twice"
- YES: "Benchmarked multiple LLM models for summarizer performance"

- NOT: "Addressed Terraform lock issues by unlocking specific resource"
- YES: "Resolved terraform state lock conflict in aws-accounts-infra"

==================== PRIORITIZATION (STRICT ORDER) ====================

1. CRITICAL: architectural decisions, major code discussions
2. HIGH: commits, PRs, major git operations
3. MEDIUM: include ONLY if needed for understanding CRITICAL/HIGH
4. LOW: include only if pattern reveals clear intent

If a lower-priority event does not add value to understanding what was accomplished, OMIT IT.

==================== HARD RULES (DO NOT BREAK THESE) ====================

NEVER use:
- "the user", "I", "we", "they"
- Uncertainty: "appears", "seems", "probably", "likely"
- Meta phrases: "worked on", "focused on", "spent time", "continued to"
- Vague actions: "made changes", "updated files", "ran commands"
- Timestamps in bullets (dates are already in event format)
- Generic accomplishments without specifics

ALWAYS use:
- Past tense action verbs
- Specific file paths when relevant
- Tool/command names when they identify the work
- Technical terminology appropriate to the domain
- Concrete outcomes when visible in events

==================== GOOD OUTPUT EXAMPLES ====================

GOOD:
Working on: devlog (main)

- Created benchmark_summarizer.sh to test qwen and llama models with different prompt variants
- Implemented automatic model unloading after tests to prevent memory exhaustion
- Fixed timestamp query bug in SQLite event fetching using unixepoch conversion

GOOD:
Working on: aws-accounts-infra (main)

- Resolved terraform state lock in wistia-dev workspace using force-unlock
- Validated infrastructure plan for ECS service updates and RDS parameter changes
- Applied terraform changes to staging environment

GOOD (mixed repos):
Working on: devlog (main)

- Discussed implementing priority-based event categorization in internal/events/event.go
- Benchmarked qwen2.5:14b for production summarizer with 50-event test cases
- Deployed configuration updates to kubernetes staging cluster

==================== OUTPUT FORMAT (STRICT) ====================

<one-line context>

- <bullet 1: most significant technical work>
- <bullet 2: second most significant work>
- <bullet 3: additional work if meaningfully different>
- <bullet 4: only if truly distinct from above>

Generate the summary now. Follow ALL rules above with zero deviations.`

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
	matches := inactivePeriodRegex.FindAllStringSubmatchIndex(string(content), -1)

	if len(matches) > 0 {
		lastMatch := matches[len(matches)-1]
		lastEndTime := string(content[lastMatch[4]:lastMatch[5]])

		expectedStartTime := focusStart.Format("15:04")
		if lastEndTime == expectedStartTime {
			updatedSection := fmt.Sprintf("## %s - %s\n\nNo development activity recorded during this period.\n\n",
				string(content[lastMatch[2]:lastMatch[3]]),
				focusEnd.Format("15:04"))

			newContent := string(content[:lastMatch[0]]) + updatedSection + string(content[lastMatch[1]:])
			return os.WriteFile(path, []byte(newContent), 0644)
		}
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

func (p *Plugin) GetLLMClient() llm.Client {
	return p.llmClient
}

func (p *Plugin) SaveSummaryExported(summary string, focusStart, focusEnd time.Time, contextEvents, focusEvents []*events.Event) error {
	return p.saveSummary(summary, focusStart, focusEnd, contextEvents, focusEvents)
}

func BuildPromptExported(contextEvents, focusEvents []*events.Event) string {
	return buildPrompt(contextEvents, focusEvents, FormatEvent)
}

func (p *Plugin) FilterEvents(evts []*events.Event) []*events.Event {
	return p.filterEvents(evts)
}

func (p *Plugin) BuildPrompt(contextEvents, focusEvents []*events.Event) string {
	return p.buildPrompt(contextEvents, focusEvents)
}
