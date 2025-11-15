DevLog — Design Document

Overview

DevLog is a local journaling system that automatically captures development activity (git hooks, shell commands, wisprflow transcripts, periodic repo scans) and generates structured LLM-powered Markdown logs inside an Obsidian vault.

It consists of:
	•	devlogd — background daemon exposing a local HTTP API
	•	devlog — CLI for ingest, diagnostics, and manual triggers
	•	Event sources — git hooks, shell hooks, wisprflow, periodic scanning
	•	Session aggregation + summarization
	•	Markdown output to daily notes in Obsidian

Goal: Hands-off dev journaling with high signal and minimal manual work.

⸻

Architecture

Components
	•	Daemon (devlogd)
	•	Runs as macOS background process.
	•	Local-only HTTP API (127.0.0.1:<port>).
	•	Stores events in SQLite.
	•	Groups events into sessions.
	•	Generates session summaries + daily summaries via LLM.
	•	Writes Markdown into Obsidian.
	•	CLI (devlog)
	•	devlog ingest
	•	devlog daemon start|stop|status
	•	devlog flush
	•	devlog status
	•	devlog tui (future)
	•	Used by hooks and external tools.
	•	Event Sources
	•	Git hooks (post-commit, post-merge, post-checkout)
	•	Shell hooks (filtered commands only)
	•	Wisprflow (speech → text)
	•	Periodic repo scan for missed commits
	•	(Future) GitHub merged PR polling


Event Model

All events follow a single schema:


```jsonc
{
  "v": 1,
  "id": "<uuid>",
  "timestamp": "ISO8601",
  "source": "git|shell|wisprflow|manual|github",
  "type": "commit|merge|command|note|pr_merged|context_switch|other",
  "repo": "optional",
  "branch": "optional",
  "payload": { "...": "source-specific" }
}
```


HTTP API

Daemon exposes:
```
POST /api/v1/ingest       # submit event
GET  /api/v1/status       # health + event count
POST /api/v1/summarize    # force summarization
```

•	Only listens on 127.0.0.1
•	Optional auth header: X-Devlog-Token: <token>


Event Sources:

- git hooks ✅
- shell hooks ✅
- wisprflow ✅
- (Future) GitHub merged PR polling

The goal here is to keep this modular and extensible.


Session Aggregation

Daemon groups events into sessions:
	•	Events < 45 min apart → same session
	•	Same repo reinforces grouping
	•	Wisprflow notes join current session
	•	Shell commands may form sessions alone if meaningful

Sessions stored in SQLite.


Summarization Logic

Session Summaries

Every 30 minutes:
	•	Gather unsummarized events in each session
	•	Build LLM prompt
	•	Generate Markdown
	•	Append to YYYY-MM-DD.md in Obsidian

Daily Summary

At 18:30 local time:
	•	Summarize all sessions from the day
	•	Insert at top of the file under:


Obsidian Output

Full file path example:
/Users/akshithyellapragada/Library/Mobile Documents/iCloud~md~obsidian/Documents/Main/Periodic/2025.11.Nov


Example Note:

```md
# 2025.11.14.Fri

## Captain's Log

- 18:11: Example log in daily notes
```

Config File

~/.config/devlog/config.yaml:

```yaml
obsidian_path: "/Users/you/Obsidian/DevLogs"
repos:
  - "~/dev/wistia/webservices"
  - "~/dev/wistia/internal"

poll_interval_minutes: 30
session_gap_minutes: 45

http:
  port: 8573
  token: ""

llm:
  provider: "anthropic"
  model: "claude-3.5-sonnet"
  api_key_env: "ANTHROPIC_API_KEY"

shell:
  enabled: true
  capture_mode: "important"   # or "all"
  ignore_list:
    - "ls"
    - "cd"
    - "pwd"
```


Potential code structure:

```sh
/cmd
  devlog
  devlogd
/internal
  config
  events
  ingest
  sessions
  scheduler
  llm
  obsidian
  shell
  git
  tui (future)
/docs
  DESIGN.md
```


Potential roadmap:

```
v0.1 – scaffold + HTTP ingest + SQLite ✅
v0.2 – git hooks + shell hooks ✅
v0.3 – session grouping
v0.4 – LLM summaries + Obsidian writing
v0.5 – daily summary
v1.0 – TUI + polish
```