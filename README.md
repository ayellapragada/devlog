<div align="center">

# 📖 DevLog

**Your automated personal development journal**

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev)
[![CI](https://github.com/ayellapragada/devlog/workflows/CI/badge.svg)](https://github.com/ayellapragada/devlog/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/ayellapragada/devlog)](https://goreportcard.com/report/github.com/ayellapragada/devlog)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

[Installation](#-installation) • [Quick Start](#-quick-start) • [Architecture](#-architecture) • [Documentation](#-documentation)

</div>

---

## 🎯 Overview

DevLog is a **privacy-first development journaling system** that automatically captures your coding activity and generates intelligent summaries of your work. Think of it as a personal time machine for your development sessions.

```
┌─────────────────────────────────────────────────────────────────┐
│  Your Development Activity                                      │
│  ━━━━━━━━━━━━━━━━━━━━━━━━━                                      │
│                                                                 │
│  git commit -m "fix: auth bug"   ──┐                            │
│  npm test                          │                            │
│  Claude: "help me optimize..."     ├────▶  Daemon               │
│  copy/paste code snippet           │         │                  │
│  tmux: switch to project-2       ──┘         │                  │
│                                              ▼                  │
│                                          SQLite DB              │
│                                              │                  │
│                                              ▼                  │
│                                         Summarization           │
│                                              │                  │
│                                              ▼                  │
│                               "Fixed authentication bug in      │
│                                user login flow. Implemented     │
│                                token refresh logic..."          │
└─────────────────────────────────────────────────────────────────┘
```

## 📦 Installation

### Prerequisites

- **Go 1.25+** - [Download here](https://go.dev/dl/)
- **Unix-like OS** - macOS or Linux (Windows support coming)

### Build from Source

```bash
git clone https://github.com/ayellapragada/devlog.git
cd devlog
make build
make install
```

This installs:
- Binary to `~/bin/devlog`
- Config directory at `~/.config/devlog/`
- SQLite database at `~/.config/devlog/events.db`

## 🚀 Quick Start

### 1. Initialize DevLog

```bash
devlog init
```

Creates configuration directory and database.

### 2. Install Modules

Choose which sources to capture from:

```bash
devlog module install git       # Git operations
devlog module install kubectl   # Kubernetes operations
devlog module install shell     # Shell commands
devlog module install clipboard # Clipboard monitoring
devlog module install tmux      # Tmux session tracking
devlog module install wisprflow # Voice transcriptions
```

### 3. Start the Daemon

```bash
devlog daemon start
```

The daemon runs in the background on `http://localhost:8573`.

### 4. Verify It's Working

```bash
# Check daemon status
devlog daemon status

# View recent events
devlog status

# Search your history
devlog search "npm install"

# See module list
devlog module list
```

### 5. (Optional) Enable Summaries

Configure an LLM provider in `~/.config/devlog/config.yaml`:

An example using local Ollama:

```yaml
plugins:
  summarizer:
    enabled: true
    provider: ollama
    base_url: http://localhost:11434
    model: qwen3:8b
    interval_minutes: 30
    context_window_minutes: 60
```

Using an anthropic key is also supported.

Summaries are automatically generated and saved to `~/.config/devlog/summaries/`.

## 🏗 Architecture

DevLog uses a **modular two-tier architecture**:

```
┌─────────────────────────────────────────────────────────────────┐
│                        DevLog Architecture                      │
└─────────────────────────────────────────────────────────────────┘

┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ Git Hook     │  │ Shell Hook   │  │ Tmux Hook    │
│ (~/.local/   │  │ (PROMPT_     │  │ (~/.tmux.    │
│  bin/git)    │  │  COMMAND)    │  │  conf)       │
└──────┬───────┘  └──────┬───────┘  └──────┬───────┘
       │                 │                 │
       └─────────────────┼─────────────────┘
                         │ HTTP POST
                         ▼
              ┌──────────────────┐
              │  DevLog Daemon   │  ◀── CLI Commands
              │  (Port 8573)     │
              └──────────────────┘
                         │
          ┌──────────────┼──────────────┐
          │              │              │
          ▼              ▼              ▼
   ┌───────────┐  ┌───────────┐  ┌───────────┐
   │  Storage  │  │  Queue    │  │  Plugins  │
   │ (SQLite)  │  │  Manager  │  │           │
   └───────────┘  └───────────┘  └─────-┬────┘
          │                             │
          │                             ▼
          │                    ┌────────────────┐
          │                    │  Summarizer    │
          │                    │  (LLM-powered) │
          │                    └────────┬───────┘
          │                             │
          └─────────────────────────────┘
                         │
                         ▼
              ┌──────────────────┐
              │  Markdown Files  │
              │  ~/.config/      │
              │   devlog/        │
              │   summaries/     │
              └──────────────────┘
```

### Components

#### 🎣 **Modules** - Event Capture
Hook-based (zero polling) examples:
- **git** - Wraps git commands to capture operations
- **shell** - Integrates with shell prompt (Bash/Zsh)

Poll-based (periodic checks):
- **clipboard** - Monitors clipboard for code snippets
- **claude** - Reads Claude Code conversation history

#### 🔌 **Plugins** - Event Processing
- **summarizer** - Automated summary generation

#### 🖥 **Daemon**
- HTTP server on localhost:8573
- Manages module pollers and plugin lifecycle
- Graceful shutdown and reload support

#### 💾 **Storage**
- SQLite database with full-text search (FTS5)

### Web Dashboard

<div align="center">
  <img src="docs/images/devlog-web-ui.png" alt="DevLog Web Dashboard" width="800">
  <p><em>Visualize your development activity with charts</em></p>
</div>

## 📚 Documentation

### Core Guides
- **[Modules](modules/README.md)** - Event capture sources and creating custom modules
- **[Plugins](plugins/README.md)** - Event processing and creating custom plugins

### Module Documentation
- [Git Module](modules/git/README.md) - Git operation capture
- [Kubectl Module](modules/kubectl/README.md) - Kubernetes operation capture
- [Shell Module](modules/shell/README.md) - Shell command tracking
- [Claude Module](modules/claude/README.md) - Claude Code integration
- [Clipboard Module](modules/clipboard/README.md) - Clipboard monitoring
- [Tmux Module](modules/tmux/README.md) - Tmux session tracking
- [Wisprflow Module](modules/wisprflow/README.md) - Voice transcription

### Plugin Documentation
- [Summarizer Plugin](plugins/summarizer/README.md) - AI-powered summaries

## 🔧 Commands Reference

### Core Commands

```bash
devlog init                          # Initialize configuration
devlog daemon start|stop|restart     # Manage daemon
devlog status [-v] [-n NUM] [-s SRC] # View recent events
devlog search <query>                # Search events
devlog web open                      # Open web dashboard in browser
```

### Configuration

```bash
devlog config status                 # Display configuration
devlog config show                   # Display config.yml
devlog config path                   # Show config file path
devlog config edit                   # Edit in $EDITOR
```

### Module Management

```bash
devlog module list                   # List available modules
devlog module install <name>         # Install and enable a module
devlog module uninstall <name>       # Uninstall a module
```

### Plugin Management

```bash
devlog plugin list                   # List available plugins
devlog plugin install <name>         # Install and enable a plugin
devlog plugin uninstall <name>       # Uninstall a plugin
```

## ⚙️ Configuration

Configuration is stored at `~/.config/devlog/config.yaml`:

```yaml
# Daemon settings
daemon:
  port: 8573

# Module configuration
modules:
  git:
    enabled: true

# Plugin configuration
plugins:
  summarizer:
    enabled: false

# Storage settings
storage:
  path: ~/.config/devlog/events.db
```

## 🛠 Development

### Building

```bash
make build          # Build binary
make install        # Install to ~/bin
make test           # Run tests
make coverage       # Test coverage report
make lint           # Run linter
```

### Project Structure

```
devlog/
├── cmd/
│   └── devlog/       # CLI binary (includes daemon functionality)
├── internal/         # Core libraries
│   ├── api/          # HTTP API handlers
│   ├── daemon/       # Daemon coordinator
│   ├── storage/      # SQLite operations
│   ├── events/       # Event definitions
│   ├── queue/        # Event queue
│   └── ...
├── modules/          # Event capture modules
│   ├── git/
│   ├── shell/
│   ├── claude/
│   └── ...
└── plugins/          # Event processing plugins
    └── summarizer/
```

## 🤝 Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.

Areas where help is appreciated:

- **Module Development** - New event sources (VS Code, Docker, etc.)
- **Plugin Development** - Export formats, integrations, analytics
- **Testing** - Platform testing, edge cases
- **Documentation** - Guides, examples, tutorials

Please feel free to submit issues or pull requests.

## 🔒 Security

DevLog stores all data locally and supports multiple LLM providers. See [SECURITY.md](SECURITY.md) for:
- Security considerations for local data storage
- API key management best practices
- How to report security vulnerabilities

## 📝 License

MIT License - see [LICENSE](LICENSE) for details.

---

## 🐛 Troubleshooting

### Daemon won't start

```bash
# Check if port is in use
lsof -i :8573

# View daemon logs
tail -f ~/.config/devlog/devlog.log
```

### Events not being captured

```bash
# Verify daemon is running
devlog daemon status

# Check module list
devlog module list

# Test manual event ingestion
devlog ingest test '{"message": "test event"}'

# View recent events
devlog status -v -n 20
```

### Modules not working

```bash
# Reinstall git module
devlog module uninstall git
devlog module install git

# Check shell integration
echo $PROMPT_COMMAND  # Should include devlog

# Verify tmux hooks
tmux show-options -g | grep devlog
```

### Summaries not generating

```bash
# Check plugin status
devlog plugin list

# View logs for errors
tail -f ~/.config/devlog/devlog.log | grep summarizer

# Test LLM connection (for Ollama)
curl http://localhost:11434/api/generate -d '{"model":"qwen3:8b","prompt":"test"}'
```

For more help, [open an issue](https://github.com/ayellapragada/devlog/issues) on GitHub.