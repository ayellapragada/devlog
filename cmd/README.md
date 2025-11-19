# cmd/

This directory contains the main entry points for DevLog's command-line binary.

## Binaries

### [devlog](devlog/)

The main CLI binary that provides all user-facing functionality.

**Location:** `cmd/devlog/main.go`

**Features:**
- Command-line interface for all devlog operations
- Module and plugin management
- Event viewing and searching
- Configuration management

**Key Responsibilities:**
- Command parsing and routing
- Daemon lifecycle management (start/stop/restart)
- Query the database
- Module installation and configuration
- Plugin installation and configuration

### [devlogd](devlogd/)

Internal daemon binary - **not for public use**. Contains duplicate daemon startup code.

**Location:** `cmd/devlogd/main.go`

**Status:** Internal implementation detail. Users should use `devlog daemon start` instead.

**Note:** This binary and `cmd/devlog/commands/daemon.go` contain duplicate implementations that both call `internal/daemon`. The CLI provides the public interface.

## Architecture

```
┌─────────────────────────────────────────┐
│  devlog (single binary)                 │
├─────────────────────────────────────────┤
│                                         │
│  CLI Commands                           │
│  ┌──────────────────────────────────┐   │
│  │ devlog status                    │   │
│  │ devlog search                    │   │
│  │ devlog config                    │   │
│  │ devlog module [enable|disable]   │   │
│  └──────────────────────────────────┘   │
│            │                            │
│            ▼                            │
│  ┌──────────────────────────────────┐   │
│  │ devlog daemon [start|stop]       │   │
│  └──────────────────────────────────┘   │
│            │                            │
│            ▼                            │
│  Daemon Process                         │
│  ┌──────────────────────────────────┐   │
│  │ HTTP Server (port 8573)          │   │
│  │ Module Pollers                   │   │
│  │ Plugin Manager                   │   │
│  │ Event Queue                      │   │
│  │ Session Tracker                  │   │
│  └──────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

## Building

```bash
make build
```

## Development

When developing commands:

1. Add new commands in `cmd/devlog/commands/`
2. Follow the command registration pattern
3. Import modules/plugins with blank imports in `main.go`
4. Use standardized error handling from `internal/errors`

See [cmd/devlog/commands/](devlog/commands/) for command implementation details.
