# cmd/devlog

The `devlog` CLI is the main user interface for interacting with the devlog system.

## Overview

The devlog CLI provides commands for:
- Initializing and configuring devlog
- Managing the daemon lifecycle
- Installing and configuring modules
- Installing and managing plugins
- Viewing captured events
- Manually ingesting events
- Managing configuration

## Directory Structure

```
cmd/devlog/
  ├── main.go           # Entry point and command registration
  ├── commands/         # Command implementations
  │   ├── init.go       # Initialize devlog
  │   ├── daemon.go     # Daemon lifecycle (start/stop/restart/status)
  │   ├── module.go     # Module management
  │   ├── plugin.go     # Plugin management
  │   ├── status.go     # View recent events
  │   ├── config.go     # Configuration management
  │   ├── ingest.go     # Manual event ingestion
  │   ├── poll.go       # Manual polling trigger
  │   ├── search.go     # Search events
  │   ├── web.go        # Web interface
  │   ├── component.go  # Shared component logic
  │   └── help.go       # Help system
  └── formatting/       # Event formatters for CLI output
      ├── events.go     # Event formatting registry
      └── formatters.go # Shared formatting utilities
```

## Key Commands

### Initialization
- `devlog init` - Initialize configuration and database

### Daemon Management
- `devlog daemon start` - Start the background daemon
- `devlog daemon stop` - Stop the daemon
- `devlog daemon restart` - Restart the daemon
- `devlog daemon status` - Check daemon status

### Module Management
- `devlog module list` - List available modules
- `devlog module install <name>` - Install and enable a module
- `devlog module uninstall [--purge] <name>` - Uninstall a module

### Plugin Management
- `devlog plugin list` - List available plugins
- `devlog plugin install <name>` - Install and enable a plugin
- `devlog plugin uninstall [--purge] <name>` - Uninstall a plugin

### Event Viewing
- `devlog status [-v] [-n NUM] [-s SOURCE]` - Show recent events
- `devlog search <query>` - Search events (future)
- `devlog web` - Launch web interface (future)

### Configuration
- `devlog config show` - Display current configuration
- `devlog config path` - Show config file location
- `devlog config edit` - Open config in editor

### Advanced
- `devlog ingest <source> <data>` - Manually ingest an event
- `devlog poll` - Trigger manual polling cycle

## Formatting System

The CLI uses a modular formatting system for displaying events. Each module registers its own formatter in [formatting/](formatting/):

- Formatters implement the `Formatter` interface
- Each module provides its own event formatting logic
- Formatters are registered during package initialization
- Allows consistent, readable output across event types

## Building

```bash
make build
```

## Installation

```bash
make install
```

This installs to `~/bin/devlog` by default.

## See Also

- [Main README](../../README.md) - Project overview and quick start
- [cmd/devlogd](../devlogd/README.md) - Daemon implementation
- [modules/](../../modules/README.md) - Available modules
- [plugins/](../../plugins/README.md) - Available plugins
