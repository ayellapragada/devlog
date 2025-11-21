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
	"devlog/internal/install"
	"devlog/internal/llm"
	"devlog/internal/plugins"
	"devlog/internal/services"
	"devlog/internal/storage"
)

type Plugin struct {
	llmClient llm.Client
}

type Config struct {
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
	Limit        int    `json:"limit"`
	ResponseGoal string `json:"response_goal"`
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
	return &Config{}
}

func (p *Plugin) ValidateConfig(config interface{}) error {
	_, ok := config.(map[string]interface{})
	if !ok {
		return errors.NewValidation("config", "must be a map")
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

func (p *Plugin) LLMClient() llm.Client {
	return p.llmClient
}

type QueryResult struct {
	Plan    *QueryPlan
	Results []*storage.SearchResult
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
	sb.WriteString(fmt.Sprintf("  Response Goal: %s\n", qr.Plan.ResponseGoal))

	return sb.String()
}

func (p *Plugin) Query(ctx context.Context, question string) (*QueryResult, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, errors.WrapPlugin("query", "load config", err)
	}

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

	eventService := services.NewEventService(store, func() *config.Config { return cfg }, nil)

	fmt.Println("Converting question to SQL query...")
	plan, err := p.generateQueryPlan(ctx, question)
	if err != nil {
		return nil, errors.WrapPlugin("query", "generate query plan", err)
	}

	fmt.Println("Searching events...")
	results, err := p.executeSearch(ctx, eventService, plan)
	if err != nil {
		return nil, errors.WrapPlugin("query", "execute search", err)
	}

	return &QueryResult{
		Plan:    plan,
		Results: results,
	}, nil
}

func (p *Plugin) generateQueryPlan(ctx context.Context, question string) (*QueryPlan, error) {
	now := time.Now()
	_, offset := now.Zone()
	tzName := now.Format("MST")
	twoHoursAgo := now.Add(-2 * time.Hour)

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
  "limit": number (choose an appropriate limit based on the question, typically 50-100),
  "response_goal": "concise description of what the user wants to know"
}

Time parsing rules:
- ALL times should use the user's local timezone: %s (offset: %+d hours from UTC)
- DEFAULT: If the question is vague about time (e.g., "what was I working on?", "what did I do?"), default to the LAST 2 HOURS (%s to now)
- "today" = start of today (00:00:00 local time) to now
- "yesterday" = start of yesterday to end of yesterday (local time)
- "last week" = 7 days ago to now
- "last Tuesday" = most recent Tuesday at 00:00:00 local time
- "11am to 3pm" = today 11:00:00 to 15:00:00 in LOCAL TIME
- "from 11am to 3pm" = today 11:00:00 to 15:00:00 in LOCAL TIME
- "past 2 hours" = 2 hours ago to now
- When no date is specified with a time, assume TODAY in local timezone
- IMPORTANT: Use the timezone offset shown above. Times like "11:00:00" should become "11:00:00%s"

Module names (sources): git, shell, kubectl, claude, tmux, clipboard, wisprflow, manual

Output ONLY valid JSON, no explanation.`,
		now.Format(time.RFC3339), tzName, offset/3600,
		now.Format("2006-01-02"),
		question,
		tzName, offset/3600,
		twoHoursAgo.Format(time.RFC3339),
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
		plan.Limit = 50
	}
	if plan.Limit > 100 {
		fmt.Println("Warning: limit is greater than 100, setting to 100")
		plan.Limit = 100
	}

	return &plan, nil
}

func (p *Plugin) executeSearch(ctx context.Context, eventService *services.EventService, plan *QueryPlan) ([]*storage.SearchResult, error) {
	query := plan.Filters.Keywords
	if query == "" {
		query = "*"
	}

	searchOpts := storage.SearchOptions{
		Query:         query,
		Limit:         plan.Limit,
		Modules:       plan.Filters.Modules,
		Types:         plan.Filters.Types,
		RepoPattern:   plan.Filters.Repo,
		BranchPattern: plan.Filters.Branch,
		After:         plan.TimeRange.Start,
		SortOrder:     storage.SortByTimeDesc,
	}

	results, err := eventService.SearchEvents(ctx, searchOpts)
	if err != nil {
		return nil, err
	}

	return results, nil
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
