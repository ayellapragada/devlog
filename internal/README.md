# internal/

This directory contains the core internal packages that power DevLog. These packages are not intended to be imported by external code.

## Packages

### api
**Location:** [internal/api/](api/)

HTTP API server and request handlers for event ingestion, status checks, and session management.

### config
**Location:** [internal/config/](config/)

Configuration file loading and management. Handles `~/.config/devlog/config.yaml`.

### daemon
**Location:** [internal/daemon/](daemon/)

Daemon lifecycle management, including startup, shutdown, and signal handling.

### events
**Location:** [internal/events/](events/)

Core event model and validation. Defines the schema for all events flowing through DevLog.

### logger
**Location:** [internal/logger/](logger/)

Structured logging utilities using Go's `log/slog` package.

### modules
**Location:** [internal/modules/](modules/)

Module registration system and installation context for pluggable modules like git and shell.

### poller
**Location:** [internal/poller/](poller/)

Polling service for external data sources (e.g., Wisprflow transcriptions).

### queue
**Location:** [internal/queue/](queue/)

Event queue management for ordered processing and buffering.

### session
**Location:** [internal/session/](session/)

Session grouping logic that clusters related events into development sessions.

### state
**Location:** [internal/state/](state/)

Global state management for daemon runtime state and session tracking.

### storage
**Location:** [internal/storage/](storage/)

SQLite database operations for events and sessions. Handles schema migrations and queries.

### testutil
**Location:** [internal/testutil/](testutil/)

Testing utilities and helpers for creating test fixtures.

## Key Dependencies

- **SQLite:** Uses `modernc.org/sqlite` for database operations
- **UUID:** Uses `github.com/google/uuid` for unique identifiers
- **Standard Library:** Extensive use of `context`, `net/http`, and `database/sql`

## Architecture Flow

```
┌─────────────┐
│   daemon    │  Coordinates components
└──────┬──────┘
       │
   ┌───┴────┬────────────┐
   │        │            │
┌──▼──┐  ┌─▼──┐    ┌────▼────┐
│ api │  │cfg │    │ storage │
└─────┘  └────┘    └─────────┘
   │                     │
   ▼                     ▼
┌────────┐          ┌────────┐
│ events │◄─────────┤session │
└────────┘          └────────┘
```

1. **daemon** initializes all components
2. **api** receives HTTP requests and validates events
3. **events** defines the data model
4. **storage** persists to SQLite
5. **session** groups events into sessions
6. **config** provides configuration to all components
