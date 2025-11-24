# Summarizer Plugin

Automatically generates intelligent summaries of your development activity using LLMs at clock-aligned intervals.

## Overview

The summarizer plugin runs on a scheduled basis (e.g., every 30 minutes at :00 and :30) and creates concise summaries that highlight:
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

## Dependencies

This plugin requires the **[llm](../llm/README.md)** plugin to be enabled and configured.

## Configuration

The summarizer plugin only configures timing - LLM settings are configured separately in the `llm` plugin.

### With Ollama (Local)

```yaml
plugins:
  llm:
    enabled: true
    provider: ollama
    base_url: http://localhost:11434
    model: qwen2.5:14b
  summarizer:
    enabled: true
    interval_seconds: 1800        # 30 minutes
    context_window_seconds: 3600  # 60 minutes
```

### With Anthropic (Cloud)

```yaml
plugins:
  llm:
    enabled: true
    provider: anthropic
    api_key: sk-ant-...
    model: claude-sonnet-4-5-20250929
  summarizer:
    enabled: true
    interval_seconds: 1800        # 30 minutes
    context_window_seconds: 3600  # 60 minutes
```

## Configuration Options

### Summarizer Options

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `interval_seconds` | int | Yes | Time interval between summaries in seconds (default: 1800 = 30 minutes, range: 60-86400) |
| `context_window_seconds` | int | Yes | Historical context window for LLM in seconds (default: 3600 = 60 minutes, range: 60-86400, must be >= interval) |
| `exclude_sources` | []string | No | Event sources to exclude from summaries (default: ["clipboard", "wisprflow"]) |

### LLM Options

See [llm plugin documentation](../llm/README.md) for LLM configuration options.

### How Time Windows Work

- **interval_seconds**: How often summaries are generated (e.g., 1800 = every 30 minutes)
- **context_window_seconds**: How far back to look for context (e.g., 3600 = past hour)

For example, with `interval_seconds: 1800` (30 min) and `context_window_seconds: 3600` (60 min):
- Summary runs at 14:30
- Focuses on activity from 14:00-14:30 (last 30 minutes)
- Uses events from 13:30-14:30 (past hour) for context

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

- **LLM plugin**: Must be enabled and configured (see [llm plugin](../llm/README.md))
- Sufficient storage for summary markdown files
- Events being captured by modules (git, shell, etc.)

## Privacy

Privacy depends on your LLM provider configuration:

- **Ollama**: All data stays local on your machine
- **Anthropic**: Events are sent to Anthropic's API for processing

See [llm plugin documentation](../llm/README.md#privacy) for more details.
