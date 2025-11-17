# DevLog

![CodeRabbit Pull Request Reviews](https://img.shields.io/coderabbit/prs/github/ayellapragada/devlog?utm_source=oss&utm_medium=github&utm_campaign=ayellapragada%2Fdevlog&labelColor=171717&color=FF570A&link=https%3A%2F%2Fcoderabbit.ai&label=CodeRabbit+Reviews)

## Overview

A local development journaling system that automatically captures development activity and stores it in a database for future review and logging.

## Key Features

- **Privacy-First**: All data stays local on your machine
- **Low Overhead**: Lightweight daemon designed to run continuously without impacting performance
- **Modular Architecture**: Extensible plugin system for event sources

## Installation

### Prerequisites

- Go 1.25 or later

### Build from Source

```bash
git clone https://github.com/ayellapragada/devlog.git
cd devlog
make build
```

This will create the `devlog` binary in the project root.

### Install

```bash
make install
```

This installs the binary to `~/bin/devlog` and sets up the necessary configuration.

## Quick Start

### 1. Initialize DevLog

```bash
devlog init
```

This creates the configuration directory (`~/.config/devlog/`) and initializes the SQLite database.

### 2. Start the Daemon

```bash
devlog daemon start
```

The daemon runs in the background on `http://localhost:9876` and begins capturing events.

### 3. Check Status

```bash
devlog status
```

View recent events captured by the system.

### 4. View Configuration

```bash
devlog config show
```

## Commands

### Core Commands

- `devlog init` - Initialize configuration and database
- `devlog daemon [start|stop|restart|status]` - Manage the daemon process
- `devlog status [-v] [-n NUM] [-s SOURCE]` - Show recent events
  - `-v, --verbose` - Show full event details
  - `-n, --number NUM` - Limit number of events (default: 10)
  - `-s, --source SOURCE` - Filter by event source (git, shell, clipboard, etc.)

### Configuration Management

- `devlog config show` - Display current configuration
- `devlog config path` - Show config file location
- `devlog config edit` - Open config file in default editor

### Module Management

- `devlog module list` - List all available modules
- `devlog module enable <name>` - Enable a module
- `devlog module disable <name>` - Disable a module
- `devlog module status` - Show module status

### Session Management

- `devlog session list` - List recent sessions
- `devlog session show <id>` - Show session details
- `devlog session current` - Show current active session

### Advanced Commands

- `devlog ingest <source> <data>` - Manually ingest an event
- `devlog poll` - Trigger manual polling cycle
- `devlog flush` - Flush event queue and session data

## Configuration

Configuration is stored in `~/.config/devlog/config.yaml`.

### Example Configuration

```yaml
daemon:
  port: 9876
  host: localhost

modules:
  git:
    enabled: true
  shell:
    enabled: true
    filter_patterns:
      - "^ls$"
      - "^cd "
  clipboard:
    enabled: true
    max_size: 10000
  tmux:
    enabled: true
  wisprflow:
    enabled: false

session:
  idle_timeout: 45m
  min_events: 3

storage:
  path: ~/.config/devlog/devlog.db
  retention_days: 90
```

## Modules

DevLog uses a modular architecture to capture different types of events:

### Available Modules

- **git** - Captures commit, merge, checkout, and other git operations via git hooks
- **shell** - Tracks important shell commands with configurable filtering
- **clipboard** - Monitors clipboard changes (deduplicated and privacy-aware)
- **tmux** - Tracks tmux session and pane switching
- **wisprflow** - Integrates with Wisprflow for speech-to-text notes

### Creating Custom Modules

See [modules/README.md](modules/README.md) for details on creating custom modules.

## Architecture

DevLog consists of several key components:

- **Daemon** - Background HTTP server that receives and processes events
- **CLI** - Command-line interface for interacting with the daemon
- **Storage** - SQLite database for event persistence
- **Modules** - Plugin system for capturing different event types
- **Session Manager** - Aggregates events into logical work sessions
- **Poller** - Periodic checks for events that can't be captured via hooks

For detailed architecture documentation, see the [ROADMAP.md](ROADMAP.md).

## Development

### Running Tests

```bash
make test
```

### Running with Coverage

```bash
make coverage
```

### Linting

```bash
make lint
```

### Building

```bash
make build
```

## Roadmap

DevLog is under active development. See [ROADMAP.md](ROADMAP.md) for planned features including:

- Session intelligence and automatic grouping
- LLM-powered summarization
- Obsidian integration
- Enhanced context awareness
- Coding activity detection
- Infrastructure and DevOps signals

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

[Add your license here]

## Privacy

DevLog is designed with privacy as a core principle:
- All data stays local on your machine
- No telemetry or external reporting
- Configurable filtering and exclusions
- Clear visibility into what's being captured

## Troubleshooting

### Daemon won't start

Check if the port is already in use:
```bash
lsof -i :9876
```

### Events not being captured

Verify the daemon is running:
```bash
devlog daemon status
```

Check module status:
```bash
devlog module status
```

### View logs

Logs are written to `~/.config/devlog/devlog.log`:
```bash
tail -f ~/.config/devlog/devlog.log
```