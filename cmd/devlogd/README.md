# cmd/devlogd

The `devlogd` daemon is the background service that powers devlog's event collection and processing capabilities.

## Overview

The devlogd daemon is the heart of the devlog system. It runs continuously in the background and:
- Receives events from modules via HTTP API
- Manages pollable modules (clipboard, wisprflow, claude)
- Executes plugins for processing and enrichment
- Stores events in SQLite database
- Manages session tracking
- Handles configuration hot-reloading

## Architecture

The daemon coordinates several key subsystems:

```
devlogd
  ├── HTTP API Server (port 8573)
  │   ├── /api/v1/ingest     - Receive events
  │   ├── /api/v1/health     - Health check
  │   └── /api/v1/config     - Dynamic config updates
  ├── Module Manager
  │   ├── Poller Registry    - Manages pollable modules
  │   └── Poller Scheduler   - Runs pollers at intervals
  ├── Plugin Manager
  │   ├── Plugin Registry    - Loaded plugins
  │   ├── Lifecycle Control  - Start/stop/restart
  │   └── Context Management - Per-plugin contexts
  ├── Storage Layer
  │   ├── SQLite Database    - Event persistence
  │   ├── Session Tracking   - Group events by sessions
  │   └── State Management   - Module state (cursors, etc)
  └── Configuration
      ├── YAML Config        - User settings
      ├── Hot Reload         - Dynamic updates
      └── Validation         - Config checking
```

## Startup Process

1. **Configuration Loading**
   - Load `~/.config/devlog/config.yaml`
   - Validate module and plugin configurations
   - Apply defaults for missing values

2. **Database Initialization**
   - Open SQLite database at `~/.config/devlog/events.db`
   - Run migrations if needed
   - Enable WAL mode for performance

3. **Module Initialization**
   - Discover enabled pollable modules
   - Create poller instances with validated configs
   - Start background polling goroutines

4. **Plugin Initialization**
   - Load enabled plugins
   - Validate plugin configurations
   - Start plugin background processes with contexts

5. **API Server Start**
   - Bind to configured port (default: 8573)
   - Start HTTP request handlers
   - Begin accepting events

6. **Graceful Shutdown Handler**
   - Register signal handlers (SIGINT, SIGTERM)
   - Setup shutdown timeouts
   - Ensure clean resource cleanup

## HTTP API

### POST /api/v1/ingest

Receive and store events from modules.

**Request Body:**
```json
{
  "v": 1,
  "id": "uuid",
  "timestamp": "2025-11-18T10:30:00Z",
  "source": "git",
  "type": "commit",
  "repo": "/path/to/repo",
  "branch": "main",
  "payload": {
    "hash": "abc123",
    "message": "Fix bug"
  }
}
```

**Response:**
```json
{
  "success": true,
  "filtered": false,
  "id": "uuid"
}
```

### GET /api/v1/health

Health check endpoint.

**Response:**
```json
{
  "status": "ok",
  "uptime": "2h15m",
  "events_processed": 1234
}
```

## Module Management

### Pollable Modules

The daemon automatically manages pollable modules:

1. **Discovery**: Enabled modules implementing `Pollable` interface
2. **Creation**: Call `CreatePoller()` with validated config
3. **Scheduling**: Run `Poll()` at configured intervals
4. **Restart**: Hot-reload on config changes
5. **Shutdown**: Clean cancellation via context

**Currently Pollable:**
- `clipboard` - Monitors system clipboard
- `wisprflow` - Polls Wisprflow database
- `claude` - Monitors Claude Code projects directory

**Hook-only Modules** (not managed by daemon):
- `git` - Events via git wrapper
- `shell` - Events via shell hooks
- `tmux` - Events via tmux hooks

### Module Lifecycle

```
Config Change
  ↓
Validate Config
  ↓
Cancel Old Poller Context
  ↓
Wait for Shutdown (5s timeout)
  ↓
Create New Poller
  ↓
Start Polling Goroutine
```

## Plugin Management

### Plugin Lifecycle

Plugins run as long-lived background services:

1. **Installation**: `plugin.Install()` - one-time setup
2. **Start**: `plugin.Start(ctx)` - spawn goroutines
3. **Running**: Plugin processes events in background
4. **Config Change**: Restart with new config
5. **Shutdown**: Cancel context, wait for cleanup (5s timeout)
6. **Uninstall**: `plugin.Uninstall()` - cleanup

### Plugin Context

Each plugin receives:
- Cancellation context for shutdown
- Configuration via context values
- Storage interface for database access
- Logger for structured logging

### Currently Available Plugins

- **summarizer** - AI-powered activity summarization
  - Runs on clock-aligned intervals
  - Uses Ollama or Anthropic
  - Generates daily markdown summaries

## Event Processing Flow

```
Module sends event
  ↓
HTTP API receives POST
  ↓
Validate event structure
  ↓
Check module filters (e.g., shell ignore list)
  ↓
Store in database (if not filtered)
  ↓
Update session tracking
  ↓
Plugins process event (async)
  ↓
Return success to module
```

## Configuration

The daemon reads configuration from `~/.config/devlog/config.yaml`:

```yaml
daemon:
  port: 8573
  log_level: info

modules:
  clipboard:
    enabled: true
    poll_interval: 3s
  claude:
    enabled: true
    poll_interval_seconds: 30

plugins:
  summarizer:
    enabled: true
    interval_minutes: 15

storage:
  path: ~/.config/devlog/events.db
  retention_days: 90
```

### Hot Reload

Some configuration changes can be applied without restart:
- Module enable/disable
- Module configuration changes
- Plugin enable/disable
- Plugin configuration changes

The daemon automatically detects config changes and:
- Restarts affected modules
- Restarts affected plugins
- Validates new configuration
- Falls back to old config on errors

## Logging

The daemon uses structured logging via `slog`:

- **Level**: Controlled by `daemon.log_level` config
- **Output**: Stdout and log file
- **Log File**: `~/.config/devlog/devlog.log`
- **Rotation**: Automatic at 10MB
- **Format**: JSON for machine parsing

**Log Levels:**
- `debug` - Detailed information for debugging
- `info` - General operational messages (default)
- `warn` - Warning conditions
- `error` - Error conditions

## Performance

The daemon is designed for minimal overhead:

- **CPU**: < 0.5% average on modern systems
- **Memory**: ~20-50MB depending on plugins
- **Disk I/O**: Minimal, WAL mode for efficiency
- **Network**: Localhost-only HTTP (< 1ms latency)

## Graceful Shutdown

On SIGINT or SIGTERM:

1. Stop accepting new HTTP requests
2. Cancel all plugin contexts
3. Cancel all poller contexts
4. Wait up to 5 seconds for cleanup
5. Close database connections
6. Exit

## Building

```bash
make build
```

## Running Manually

For development/testing:

```bash
./devlogd
```

The daemon runs in the foreground and logs to stdout.

## Production Deployment

Use the CLI for daemon management:

```bash
devlog daemon start    # Background process
devlog daemon stop     # Graceful shutdown
devlog daemon restart  # Stop then start
devlog daemon status   # Check if running
```

## Process Management

The daemon is managed as a background process:
- PID file: `~/.config/devlog/devlogd.pid`
- Log file: `~/.config/devlog/devlog.log`
- Auto-restart on crash: Not implemented (future)

## Troubleshooting

### Daemon won't start

**Check port availability:**
```bash
lsof -i :8573
```

**Check logs:**
```bash
tail -f ~/.config/devlog/devlog.log
```

**Verify config:**
```bash
devlog config show
```

### High CPU usage

1. Check which modules are enabled
2. Review plugin configurations
3. Increase polling intervals
4. Check for plugin errors in logs

### Database locked errors

SQLite WAL mode should prevent this, but if it occurs:
1. Stop daemon
2. Check for stale processes: `ps aux | grep devlogd`
3. Remove lock files: `rm ~/.config/devlog/*.db-shm ~/.config/devlog/*.db-wal`
4. Restart daemon

## Development

### Adding New Modules

Modules auto-register during init(). Import in [main.go](main.go):

```go
import (
    _ "devlog/modules/yourmodule"
)
```

### Adding New Plugins

Plugins auto-register during init(). Import in [main.go](main.go):

```go
import (
    _ "devlog/plugins/yourplugin"
)
```

## See Also

- [Main README](../../README.md) - Project overview
- [cmd/devlog](../devlog/README.md) - CLI tool
- [internal/daemon](../../internal/daemon/) - Daemon implementation
- [modules/](../../modules/README.md) - Available modules
- [plugins/](../../plugins/README.md) - Available plugins
