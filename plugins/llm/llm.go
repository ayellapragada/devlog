package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"devlog/internal/contextkeys"
	"devlog/internal/errors"
	"devlog/internal/install"
	"devlog/internal/llm"
	"devlog/internal/plugins"
)

type Plugin struct {
	client llm.Client
}

type Config struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	BaseURL  string `json:"base_url,omitempty"`
	Model    string `json:"model,omitempty"`
}

func init() {
	plugins.Register(&Plugin{})
}

func (p *Plugin) Name() string {
	return "llm"
}

func (p *Plugin) Description() string {
	return "Provides LLM client service to other plugins"
}

func (p *Plugin) Metadata() plugins.Metadata {
	return plugins.Metadata{
		Name:         "llm",
		Description:  "Provides LLM client service to other plugins",
		Dependencies: []string{},
	}
}

func (p *Plugin) Install(ctx *install.Context) error {
	ctx.Log("Installing LLM plugin")
	ctx.Log("This plugin provides LLM services to other plugins")
	ctx.Log("Configure your provider (ollama or anthropic) in the plugin configuration")
	return nil
}

func (p *Plugin) Uninstall(ctx *install.Context) error {
	ctx.Log("Uninstalling LLM plugin")
	return nil
}

func (p *Plugin) DefaultConfig() interface{} {
	return &Config{
		Provider: "ollama",
		BaseURL:  "http://localhost:11434",
		Model:    "qwen2.5:14b",
	}
}

func (p *Plugin) ValidateConfig(config interface{}) error {
	cfgMap, ok := config.(map[string]interface{})
	if !ok {
		return errors.NewValidation("config", "must be a map")
	}

	provider, ok := cfgMap["provider"].(string)
	if !ok || provider == "" {
		return errors.NewValidation("provider", "is required")
	}

	if provider != "ollama" && provider != "anthropic" {
		return errors.NewValidation("provider", "must be 'ollama' or 'anthropic'")
	}

	if provider == "anthropic" {
		apiKey, ok := cfgMap["api_key"].(string)
		if !ok || apiKey == "" {
			return errors.NewValidation("api_key", "is required for anthropic provider")
		}
	}

	if provider == "ollama" {
		baseURL, ok := cfgMap["base_url"].(string)
		if !ok || baseURL == "" {
			return errors.NewValidation("base_url", "is required for ollama provider")
		}
	}

	return nil
}

func (p *Plugin) Initialize(ctx context.Context) error {
	cfgMap, ok := ctx.Value(contextkeys.PluginConfig).(map[string]interface{})
	if !ok || cfgMap == nil {
		return errors.WrapPlugin("llm", "initialize", fmt.Errorf("plugin config not found in context"))
	}

	cfg := &Config{}
	cfgBytes, err := json.Marshal(cfgMap)
	if err != nil {
		return errors.WrapPlugin("llm", "marshal config", err)
	}
	if err := json.Unmarshal(cfgBytes, cfg); err != nil {
		return errors.WrapPlugin("llm", "unmarshal config", err)
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

	client, err := llm.NewClient(llmCfg)
	if err != nil {
		return errors.WrapPlugin("llm", "create client", err)
	}
	p.client = client
	return nil
}

func (p *Plugin) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (p *Plugin) Services() map[string]interface{} {
	return map[string]interface{}{
		"llm.client": p.client,
	}
}
