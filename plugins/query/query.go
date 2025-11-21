package query

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"devlog/internal/config"
	"devlog/internal/errors"
	"devlog/internal/events"
	"devlog/internal/install"
	"devlog/internal/llm"
	"devlog/internal/plugins"
	"devlog/internal/storage"
	"devlog/plugins/summarizer"
)

type Plugin struct {
	llmClient llm.Client
}

type Config struct {
	MaxResults int `json:"max_results"`
}

type QueryPlan struct {
	TimeRange struct {
		Start *time.Time `json:"start,omitempty"`
		End   *time.Time `json:"end,omitempty"`
	} `json:"time_range"`
	Filters struct {
		Modules  []string `json:"modules,omitempty"`
		Types    []string `json:"types,omitempty"`
		Repo     string   `json:"repo,omitempty"`
		Branch   string   `json:"branch,omitempty"`
		Keywords string   `json:"keywords,omitempty"`
	} `json:"filters"`
	Limit          int    `json:"limit"`
	FormatResponse bool   `json:"format_response"`
	ResponseGoal   string `json:"response_goal"`
}

func init() {
	plugins.Register(&Plugin{})
}

func (p *Plugin) Name() string {
	return "query"
}

func (p *Plugin) Description() string {
	return "Natural language interface to query development history using an LLM"
}

func (p *Plugin) Metadata() plugins.Metadata {
	return plugins.Metadata{
		Name:         "query",
		Description:  "Natural language interface to query development history using an LLM",
		Dependencies: []string{"llm"},
	}
}

func (p *Plugin) Install(ctx *install.Context) error {
	ctx.Log("Installing Query plugin")
	ctx.Log("This plugin requires the 'llm' plugin to be enabled")
	ctx.Log("Use 'devlog query <question>' to ask questions about your dev history")
	return nil
}

func (p *Plugin) Uninstall(ctx *install.Context) error {
	ctx.Log("Uninstalling Query plugin")
	return nil
}

func (p *Plugin) DefaultConfig() interface{} {
	return &Config{
		MaxResults: 100,
	}
}

func (p *Plugin) ValidateConfig(config interface{}) error {
	cfgMap, ok := config.(map[string]interface{})
	if !ok {
		return errors.NewValidation("config", "must be a map")
	}

	var maxResults float64
	if val, ok := cfgMap["max_results"]; ok {
		switch v := val.(type) {
		case float64:
			maxResults = v
		case int:
			maxResults = float64(v)
		default:
			return errors.NewValidation("max_results", "must be a number")
		}
		if maxResults < 1 || maxResults > 10000 {
			return errors.NewValidation("max_results", "must be between 1 and 10000")
		}
	}

	return nil
}

func (p *Plugin) InjectServices(services map[string]interface{}) error {
	llmClient, ok := services["llm.client"]
	if !ok {
		return errors.WrapPlugin("query", "inject services", fmt.Errorf("llm.client service not found"))
	}

	client, ok := llmClient.(llm.Client)
	if !ok {
		return errors.WrapPlugin("query", "inject services", fmt.Errorf("llm.client service has wrong type"))
	}

	p.llmClient = client
	return nil
}

func (p *Plugin) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

type QueryResult struct {
	Plan   *QueryPlan
	Answer string
}

func (qr *QueryResult) FormatPlan() string {
	var sb strings.Builder
	sb.WriteString("Query Plan:\n")

	if qr.Plan.TimeRange.Start != nil || qr.Plan.TimeRange.End != nil {
		sb.WriteString("  Time Range: ")
		if qr.Plan.TimeRange.Start != nil {
			sb.WriteString(fmt.Sprintf("from %s ", qr.Plan.TimeRange.Start.Format("2006-01-02 15:04:05 MST")))
		}
		if qr.Plan.TimeRange.End != nil {
			sb.WriteString(fmt.Sprintf("to %s", qr.Plan.TimeRange.End.Format("2006-01-02 15:04:05 MST")))
		}
		sb.WriteString("\n")
	}

	if len(qr.Plan.Filters.Modules) > 0 {
		sb.WriteString(fmt.Sprintf("  Modules: %s\n", strings.Join(qr.Plan.Filters.Modules, ", ")))
	}

	if len(qr.Plan.Filters.Types) > 0 {
		sb.WriteString(fmt.Sprintf("  Types: %s\n", strings.Join(qr.Plan.Filters.Types, ", ")))
	}

	if qr.Plan.Filters.Repo != "" {
		sb.WriteString(fmt.Sprintf("  Repo: %s\n", qr.Plan.Filters.Repo))
	}

	if qr.Plan.Filters.Branch != "" {
		sb.WriteString(fmt.Sprintf("  Branch: %s\n", qr.Plan.Filters.Branch))
	}

	if qr.Plan.Filters.Keywords != "" {
		sb.WriteString(fmt.Sprintf("  Keywords: %s\n", qr.Plan.Filters.Keywords))
	}

	sb.WriteString(fmt.Sprintf("  Limit: %d\n", qr.Plan.Limit))

	if qr.Plan.FormatResponse {
		sb.WriteString(fmt.Sprintf("  Response Mode: Formatted (goal: %s)\n", qr.Plan.ResponseGoal))
	} else {
		sb.WriteString("  Response Mode: Raw event list\n")
	}

	return sb.String()
}

func (p *Plugin) Query(ctx context.Context, question string, maxResults int) (*QueryResult, error) {
	dataDir, err := config.DataDir()
	if err != nil {
		return nil, errors.WrapPlugin("query", "get data dir", err)
	}
	dbPath := filepath.Join(dataDir, "events.db")

	store, err := storage.New(dbPath)
	if err != nil {
		return nil, errors.WrapPlugin("query", "open storage", err)
	}
	defer store.Close()

	plan, err := p.generateQueryPlan(ctx, question, maxResults)
	if err != nil {
		return nil, fmt.Errorf("generate query plan: %w", err)
	}

	results, err := p.executeQuery(ctx, store, plan)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}

	var answer string
	switch {
	case len(results) == 0:
		answer = "No events found matching your query."
	case plan.FormatResponse:
		answer, err = p.formatResponse(ctx, results, plan.ResponseGoal)
		if err != nil {
			return nil, err
		}
	default:
		answer = p.formatEvents(results)
	}

	return &QueryResult{
		Plan:   plan,
		Answer: answer,
	}, nil
}

func (p *Plugin) generateQueryPlan(ctx context.Context, question string, maxResults int) (*QueryPlan, error) {
	now := time.Now()
	_, offset := now.Zone()
	tzName := now.Format("MST")

	prompt := fmt.Sprintf(`You are a query planner for a development activity database. Convert the user's natural language question into a structured query plan.

Current time: %s (timezone: %s, offset: %+d hours)
Current date: %s

User question: %s

Analyze the question and generate a JSON query plan with these fields:

{
  "time_range": {
    "start": "RFC3339 timestamp with timezone or null",
    "end": "RFC3339 timestamp with timezone or null"
  },
  "filters": {
    "modules": ["git", "shell", "claude", etc] or null for all,
    "types": ["commit", "command", etc] or null for all,
    "repo": "repository name pattern" or null,
    "branch": "branch name pattern" or null,
    "keywords": "search keywords" or null
  },
  "limit": number (default %d, max %d),
  "format_response": boolean - true if answer needs LLM formatting, false for raw event list,
  "response_goal": "what the user wants to know" - used for formatting if needed
}

Time parsing rules:
- ALL times should use the user's local timezone: %s (offset: %+d hours from UTC)
- "today" = start of today (00:00:00 local time) to now
- "yesterday" = start of yesterday to end of yesterday (local time)
- "last week" = 7 days ago to now
- "last Tuesday" = most recent Tuesday at 00:00:00 local time
- "11am to 3pm" = today 11:00:00 to 15:00:00 in LOCAL TIME
- "from 11am to 3pm" = today 11:00:00 to 15:00:00 in LOCAL TIME
- "past 2 hours" = 2 hours ago to now
- When no date is specified with a time, assume TODAY in local timezone
- IMPORTANT: Use the timezone offset shown above. Times like "11:00:00" should become "11:00:00%s"

Module names (sources): git, shell, kubectl, claude, github, clipboard, tmux, wisprflow, manual

Response formatting:
- Use format_response=true for questions like "what was I working on", "summarize my work"
- Use format_response=false for "show me", "list", "find" questions

Output ONLY valid JSON, no explanation.`,
		now.Format(time.RFC3339), tzName, offset/3600,
		now.Format("2006-01-02"),
		question,
		maxResults, maxResults,
		tzName, offset/3600,
		now.Format("-07:00"))

	responseStr, err := p.llmClient.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("llm completion failed: %w", err)
	}

	responseStr = strings.TrimSpace(responseStr)
	if strings.HasPrefix(responseStr, "```json") {
		responseStr = strings.TrimPrefix(responseStr, "```json")
		responseStr = strings.TrimSuffix(responseStr, "```")
		responseStr = strings.TrimSpace(responseStr)
	} else if strings.HasPrefix(responseStr, "```") {
		responseStr = strings.TrimPrefix(responseStr, "```")
		responseStr = strings.TrimSuffix(responseStr, "```")
		responseStr = strings.TrimSpace(responseStr)
	}

	var plan QueryPlan
	if err := json.Unmarshal([]byte(responseStr), &plan); err != nil {
		return nil, fmt.Errorf("parse query plan: %w (response: %s)", err, responseStr)
	}

	if plan.Limit <= 0 {
		plan.Limit = maxResults
	}
	if plan.Limit > maxResults {
		plan.Limit = maxResults
	}

	return &plan, nil
}

func (p *Plugin) executeQuery(ctx context.Context, store *storage.Storage, plan *QueryPlan) ([]*events.Event, error) {
	if plan.Filters.Keywords != "" {
		return p.executeSearch(ctx, store, plan)
	}
	return p.executeSimpleQuery(ctx, store, plan)
}

func (p *Plugin) executeSimpleQuery(ctx context.Context, store *storage.Storage, plan *QueryPlan) ([]*events.Event, error) {
	var allEvents []*events.Event

	modules := plan.Filters.Modules
	if len(modules) == 0 {
		modules = []string{""}
	}

	for _, module := range modules {
		opts := storage.QueryOptions{
			StartTime: plan.TimeRange.Start,
			EndTime:   plan.TimeRange.End,
			Source:    module,
			Limit:     plan.Limit,
		}

		events, err := store.QueryEventsContext(ctx, opts)
		if err != nil {
			return nil, err
		}
		allEvents = append(allEvents, events...)
	}

	filtered := p.applyFilters(allEvents, plan)

	if len(filtered) > plan.Limit {
		filtered = filtered[:plan.Limit]
	}

	return filtered, nil
}

func (p *Plugin) executeSearch(ctx context.Context, store *storage.Storage, plan *QueryPlan) ([]*events.Event, error) {
	searchOpts := storage.SearchOptions{
		Query:         plan.Filters.Keywords,
		Limit:         plan.Limit,
		Modules:       plan.Filters.Modules,
		Types:         plan.Filters.Types,
		RepoPattern:   plan.Filters.Repo,
		BranchPattern: plan.Filters.Branch,
		After:         plan.TimeRange.Start,
		SortOrder:     storage.SortByTimeDesc,
	}

	results, err := store.Search(ctx, searchOpts)
	if err != nil {
		return nil, err
	}

	evts := make([]*events.Event, len(results))
	for i, r := range results {
		evts[i] = r.Event
	}

	return evts, nil
}

func (p *Plugin) applyFilters(evts []*events.Event, plan *QueryPlan) []*events.Event {
	var filtered []*events.Event

	for _, evt := range evts {
		if len(plan.Filters.Types) > 0 {
			matched := false
			for _, t := range plan.Filters.Types {
				if evt.Type == t {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		if plan.Filters.Repo != "" && !strings.Contains(strings.ToLower(evt.Repo), strings.ToLower(plan.Filters.Repo)) {
			continue
		}

		if plan.Filters.Branch != "" && !strings.Contains(strings.ToLower(evt.Branch), strings.ToLower(plan.Filters.Branch)) {
			continue
		}

		filtered = append(filtered, evt)
	}

	return filtered
}

func (p *Plugin) formatResponse(ctx context.Context, events []*events.Event, goal string) (string, error) {
	eventsBySource := groupEventsBySource(events)

	prompt := fmt.Sprintf(`You are answering a question about development activity. Use ONLY the information in the events provided.

User's question goal: %s

Events found:
%s

Instructions:
- Answer the user's question directly and concisely
- Use specific details from events (times, files, commands, repos, branches)
- Use past tense for completed actions
- Be factual - don't guess or infer beyond what's shown
- If events don't fully answer the question, say so

Generate your answer now.`, goal, formattedBySource(eventsBySource))

	answer, err := p.llmClient.Complete(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("format response: %w", err)
	}

	return strings.TrimSpace(answer), nil
}

func (p *Plugin) formatEvents(events []*events.Event) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d events:\n\n", len(events)))

	for _, evt := range events {
		sb.WriteString(summarizer.FormatEvent(evt))
		sb.WriteString("\n")
	}

	return sb.String()
}

func groupEventsBySource(evts []*events.Event) map[string][]*events.Event {
	grouped := make(map[string][]*events.Event)
	for _, evt := range evts {
		grouped[evt.Source] = append(grouped[evt.Source], evt)
	}
	return grouped
}

func formattedBySource(eventsBySource map[string][]*events.Event) string {
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
			sb.WriteString(summarizer.FormatEvent(evt) + "\n")
		}
	}

	return sb.String()
}

func LoadPlugin() (*Plugin, *Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}

	if !cfg.IsPluginEnabled("query") {
		return nil, nil, fmt.Errorf("query plugin is not enabled")
	}

	pluginCfg := &Config{}
	if rawCfg, ok := cfg.GetPluginConfig("query"); ok {
		cfgBytes, err := json.Marshal(rawCfg)
		if err != nil {
			return nil, nil, errors.WrapPlugin("query", "marshal config", err)
		}
		if err := json.Unmarshal(cfgBytes, pluginCfg); err != nil {
			return nil, nil, errors.WrapPlugin("query", "unmarshal config", err)
		}
	}

	plugin := &Plugin{}

	llmClient, err := loadLLMClient()
	if err != nil {
		return nil, nil, err
	}

	services := map[string]interface{}{
		"llm.client": llmClient,
	}

	if err := plugin.InjectServices(services); err != nil {
		return nil, nil, err
	}

	return plugin, pluginCfg, nil
}

func loadLLMClient() (llm.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	if !cfg.IsPluginEnabled("llm") {
		return nil, fmt.Errorf("llm plugin is not enabled")
	}

	llmCfgMap, ok := cfg.GetPluginConfig("llm")
	if !ok {
		return nil, fmt.Errorf("llm plugin config not found")
	}

	llmCfg := &struct {
		Provider string `json:"provider"`
		APIKey   string `json:"api_key"`
		BaseURL  string `json:"base_url,omitempty"`
		Model    string `json:"model,omitempty"`
	}{}

	cfgBytes, err := json.Marshal(llmCfgMap)
	if err != nil {
		return nil, errors.WrapPlugin("query", "marshal llm config", err)
	}
	if err := json.Unmarshal(cfgBytes, llmCfg); err != nil {
		return nil, errors.WrapPlugin("query", "unmarshal llm config", err)
	}

	client, err := llm.NewClient(llm.Config{
		Provider: llm.ProviderType(llmCfg.Provider),
		APIKey:   llmCfg.APIKey,
		BaseURL:  llmCfg.BaseURL,
		Model:    llmCfg.Model,
	})
	if err != nil {
		return nil, errors.WrapPlugin("query", "create llm client", err)
	}

	return client, nil
}
