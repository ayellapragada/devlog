# cmd/

This directory contains the main entry points for DevLog's command-line binaries.

## Binaries

### devlog
**Location:** [cmd/devlog/main.go](devlog/main.go)

The primary CLI tool for interacting with DevLog. This binary provides all user-facing commands for managing the daemon, ingesting events, viewing status, and configuring modules.

**Key Features:**
- Daemon lifecycle management (start, stop, status)
- Event ingestion from various sources
- Status reporting and event history viewing
- Module installation/uninstallation (git, shell)
- Session management
- Configuration management

**Commands:** See [cmd/devlog/commands/](devlog/commands/) for detailed command implementations.

### devlogd
**Location:** [cmd/devlogd/main.go](devlogd/main.go)

The DevLog daemon that runs as a background service. This binary starts the HTTP server, initializes storage, and handles incoming events from hooks and other sources.

**Responsibilities:**
- Start and run the HTTP API server on port 8573
- Initialize SQLite database connection
- Handle graceful shutdown
- Coordinate between storage, API, and session management

## Building

```bash
# Build the CLI
go build -o bin/devlog ./cmd/devlog

# Build the daemon
go build -o bin/devlogd ./cmd/devlogd
```

## Architecture

```
cmd/
├── devlog/          # CLI binary
│   ├── main.go      # Entry point, command routing
│   ├── commands/    # Command implementations
│   └── formatting/  # Output formatting utilities
└── devlogd/         # Daemon binary
    └── main.go      # Daemon entry point
```

Both binaries are registered with modules ([modules/git](../modules/git/) and [modules/shell](../modules/shell/)) to enable module installation capabilities.
