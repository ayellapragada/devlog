# LLM Plugin

Provides LLM (Large Language Model) client services to other plugins that need AI capabilities.

## Overview

The LLM plugin is a **service provider plugin** that manages LLM client configuration and exposes a shared client instance to other plugins. This allows multiple plugins to use AI features without duplicating LLM setup code.

## Features

- **Multiple providers**: Supports Ollama (local) and Anthropic (cloud)
- **Service provider**: Exposes `llm.client` service to dependent plugins
- **Centralized configuration**: Single source of truth for LLM settings
- **Plugin dependency management**: Other plugins can declare dependency on `llm`

## Configuration

### Ollama (Local)

```yaml
plugins:
  llm:
    enabled: true
    provider: ollama
    base_url: http://localhost:11434
    model: qwen2.5:14b
```

### Anthropic (Cloud)

```yaml
plugins:
  llm:
    enabled: true
    provider: anthropic
    api_key: sk-ant-...
    model: claude-sonnet-4-5-20250929
```

## Configuration Options

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `provider` | string | Yes | LLM provider: `ollama` or `anthropic` |
| `api_key` | string | For Anthropic | API key for Anthropic service |
| `base_url` | string | For Ollama | Ollama server URL |
| `model` | string | No | Model name (provider-specific defaults) |

## Installation

```bash
devlog plugin install llm
```

Then configure your preferred provider in `~/.config/devlog/config.yaml`.

## Dependent Plugins

Plugins that require LLM services declare a dependency:

```go
func (p *YourPlugin) Metadata() plugins.Metadata {
    return plugins.Metadata{
        Name:         "yourplugin",
        Dependencies: []string{"llm"},  // Requires LLM plugin
    }
}
```

The daemon automatically:
1. Starts the LLM plugin first
2. Registers the `llm.client` service
3. Injects the service into dependent plugins
4. Fails if LLM plugin is not enabled

## Current Consumers

- **[summarizer](../summarizer/README.md)**: Uses LLM to generate activity summaries

## Requirements

### For Ollama
- Ollama running locally or accessible via network
- Compatible model pulled (e.g., `ollama pull qwen2.5:14b`)

### For Anthropic
- Valid Anthropic API key ([get one here](https://console.anthropic.com/))
- Internet connection
- API usage quota

## Supported Models

### Ollama
Any model compatible with Ollama's API:
- `qwen2.5:14b` (recommended, good balance)
- `llama3.1:8b` (faster, lighter)
- `mistral:7b`
- Or any custom model you've pulled

### Anthropic
- `claude-sonnet-4-5-20250929` (recommended)
- `claude-opus-4-5-20250929` (most capable)
- `claude-haiku-4-5-20251001` (fastest, cheapest)

## Privacy

- **Ollama**: All data stays local on your machine
- **Anthropic**: Events are sent to Anthropic's API for processing

## Service Interface

The LLM plugin exposes this service:

```go
type Client interface {
    Complete(ctx context.Context, prompt string) (string, error)
}
```

Plugins receive this via service injection:

```go
func (p *YourPlugin) InjectServices(services map[string]interface{}) error {
    llmClient, ok := services["llm.client"]
    if !ok {
        return fmt.Errorf("llm.client service not found")
    }

    client, ok := llmClient.(llm.Client)
    if !ok {
        return fmt.Errorf("llm.client has wrong type")
    }

    p.llmClient = client
    return nil
}
```

## Troubleshooting

### "llm plugin is not enabled"

Enable the LLM plugin in your config:

```yaml
plugins:
  llm:
    enabled: true
    # ... rest of config
```

### "failed to connect to Ollama"

Check that Ollama is running:

```bash
curl http://localhost:11434/api/tags
```

If not running, start it:

```bash
ollama serve
```

### "API key invalid" (Anthropic)

Verify your API key is correct and has quota:
- Check [Anthropic Console](https://console.anthropic.com/)
- Ensure key starts with `sk-ant-`
- Check usage limits

## See Also

- [Plugin Architecture](../README.md)
- [Summarizer Plugin](../summarizer/README.md)
- [Anthropic API Docs](https://docs.anthropic.com/)
- [Ollama Documentation](https://ollama.ai/)
