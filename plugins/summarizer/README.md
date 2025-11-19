# Summarizer Plugin

Automatically generates intelligent summaries of your development activity using LLMs at clock-aligned intervals.

## Overview

The summarizer plugin runs on a scheduled basis (e.g., every 15 minutes at :00, :15, :30, :45) and creates concise summaries that highlight:
- What you were working on during the time period
- Key actions taken and commits made
- Context switches between tasks
- Development patterns and workflows

Summaries are organized into daily markdown files with collapsible event details, making it easy to review your day's work.

## Features

- **Clock-aligned scheduling**: Summaries run at predictable times (:00, :15, :30, :45)
- **Time-windowed analysis**: Uses historical context (e.g., past hour) to understand recent activity
- **Daily markdown files**: One file per day with time-stamped sections
- **Multiple LLM providers**: Support for Ollama (local) and Anthropic (cloud)
- **Collapsible event details**: Clean summaries with expandable event lists
- **Empty period handling**: Creates placeholders when no activity is detected

## Configuration

### Ollama (Local)

```yaml
plugins:
  summarizer:
    enabled: true
    provider: ollama
    base_url: http://localhost:11434
    model: qwen3:8b
    interval_minutes: 15
    context_window_minutes: 60
```

### Anthropic (Cloud)

```yaml
plugins:
  summarizer:
    enabled: true
    provider: anthropic
    api_key: your-api-key-here
    model: claude-haiku-4-5-20251001
    interval_minutes: 15
    context_window_minutes: 60
```

## Configuration Options

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `provider` | string | Yes | LLM provider: `ollama` or `anthropic` |
| `api_key` | string | For Anthropic | API key for Anthropic service |
| `base_url` | string | For Ollama | Ollama server URL (default: `http://localhost:11434`) |
| `model` | string | No | Model name (defaults: `qwen3:8b` for Ollama, `claude-haiku-4-5-20251001` for Anthropic) |
| `interval_minutes` | int | Yes | Time interval between summaries (default: 15, range: 1-1440) |
| `context_window_minutes` | int | Yes | Historical context window for LLM (default: 60, range: 1-1440, must be >= interval) |

### How Time Windows Work

- **interval_minutes**: How often summaries are generated (e.g., 15 = every 15 minutes)
- **context_window_minutes**: How far back to look for context (e.g., 60 = past hour)

For example, with `interval_minutes: 15` and `context_window_minutes: 60`:
- Summary runs at 14:45
- Focuses on activity from 14:30-14:45 (last 15 minutes)
- Uses events from 13:45-14:45 (past hour) for context

## Installation

```bash
devlog plugin install summarizer
```

Then configure your preferred provider in `~/.config/devlog/config.yaml`.

## Output

Summaries are saved to: `~/.local/share/devlog/summaries/summary_YYYY-MM-DD.md`

Each daily file contains multiple time-stamped sections:

```markdown
# Development Summary - November 17, 2025

## 14:30 - 14:45

Refactored the summarizer plugin to use time-based windows instead of
event counts. Implemented clock-aligned scheduling and added daily
markdown file generation.

<details>
<summary>Events processed (8 events)</summary>

- `14:32` **git/commit** (repo: devlog, branch: main): Refactor summarizer
- `14:35` **shell/command**: go test ./...
...

</details>

---
```

## Use Cases

- **End-of-day reviews**: Understand what you accomplished
- **Standup preparation**: Quick recap of recent work
- **Context recovery**: Return to work after interruptions
- **Time tracking**: See where your development time went
- **Pattern recognition**: Identify workflow inefficiencies

## Requirements

### For Ollama
- Ollama running locally or accessible via network
- A compatible model pulled and ready

### For Anthropic
- Valid Anthropic API key
- Internet connection
- API usage quota

## Privacy

- **Ollama**: All data stays local on your machine
- **Anthropic**: Events are sent to Anthropic's API for processing
