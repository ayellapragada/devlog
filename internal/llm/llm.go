package llm

import "context"

type Client interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

type ProviderType string

const (
	ProviderAnthropic ProviderType = "anthropic"
	ProviderOllama    ProviderType = "ollama"
)

type Config struct {
	Provider ProviderType
	APIKey   string
	BaseURL  string
	Model    string
}

func NewClient(cfg Config) (Client, error) {
	switch cfg.Provider {
	case ProviderOllama:
		return newOllamaClient(cfg.BaseURL, cfg.Model), nil
	case ProviderAnthropic:
		return newAnthropicClient(cfg.APIKey, cfg.Model), nil
	default:
		return nil, nil
	}
}
