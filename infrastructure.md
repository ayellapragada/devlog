# devlog Infrastructure Architecture

## Overview

devlog uses a three-tier architecture to handle event capture, processing infrastructure, and output destinations.

## Three-Tier Architecture

### Modules (Event Sources)
**Purpose**: "What happened?"

Modules capture events from various sources and ingest them into the devlog system.

**Examples:**
- tmux (session, window, pane events)
- clipboard (copy/paste tracking)
- git (repository activity)
- shell (command history)
- wisprflow (voice transcription)

**Characteristics:**
- May use plugins for infrastructure needs (e.g., webhook module using ngrok plugin)
- Self-register using `init()` pattern
- Implement common Module interface

### Plugins (Infrastructure/Services)
**Purpose**: "How do we make it work?"

Plugins provide capabilities and services to the system. They run as long-lived services (goroutines) that start with devlog and provide functionality to modules and exporters.

**Examples:**
- **ngrok**: Tunnel for local webhook development
- **Cloud sync**: S3, Dropbox, etc. for syncing logs
- **Notifications**: Slack, Discord, email alerts
- **LLM processing**: OpenAI, Anthropic for summarization/analysis
- **Auth providers**: OAuth services if needed
- **Search indexing**: Elasticsearch, Meilisearch for searchable logs

**Characteristics:**
- Run as goroutines (lightweight, in-process)
- Start automatically on devlog startup
- Accessible via shared registry (like modules)
- Provide services that modules/exporters can consume

### Exporters (Output Destinations)
**Purpose**: "Where does it go?"

Exporters write or send events to various destinations. They are triggered either periodically (batched) or on-demand via CLI.

**Examples:**
- **Markdown files**: Basic file output
- **Obsidian**: Markdown to specific vault location
- **Database**: Postgres, SQLite
- **Time-tracking**: Toggl, Clockify integration
- **Note apps**: Notion, Roam Research

**Characteristics:**
- Triggered periodically (batch processing)
- Triggered on-demand via CLI (`devlog export`)
- Process batches of events
- Self-register using similar pattern to modules

## Technical Implementation

### Interface Definitions

#### Module Interface (Existing)
```go
type Module interface {
    Name() string
    Description() string
    Install(ctx *InstallContext) error
    Uninstall(ctx *InstallContext) error
    DefaultConfig() interface{}
    ValidateConfig(config interface{}) error
}
```

#### Plugin Interface (New)
```go
type Plugin interface {
    Name() string
    Description() string
    Start(ctx context.Context) error   // Run until context cancelled
    DefaultConfig() interface{}
    ValidateConfig(config interface{}) error
}
```

#### Exporter Interface (New)
```go
type Exporter interface {
    Name() string
    Description() string
    Initialize(ctx *Context) error     // One-time setup
    Export(events []Event) error       // Process batch of events
    Shutdown() error                   // Cleanup
    DefaultConfig() interface{}
    ValidateConfig(config interface{}) error
}
```

### Lifecycle Flow

1. **Startup**: devlog starts → all enabled plugins Start() in goroutines
2. **Event Collection**: Events collected by modules → stored in buffer
3. **Periodic Export**: Every N minutes → batch export via exporters
4. **Manual Export**: User runs `devlog export` → manual export via exporters
5. **Shutdown**: devlog stops → plugins receive context cancellation, shutdown gracefully

### Configuration Structure

Configuration file example:
```yaml
modules:
  - name: tmux
    enabled: true
    config: {...}
  - name: clipboard
    enabled: true
    config: {...}

plugins:
  - name: ngrok
    enabled: true
    config: {...}
  - name: notifications
    enabled: false
    config: {...}

exporters:
  - name: markdown
    enabled: true
    config: {...}
  - name: obsidian
    enabled: false
    config: {...}
```

## Design Decisions

### Goroutines vs Separate Processes
**Decision**: Use goroutines for plugins

**Rationale:**
- Lightweight and fits Go's concurrency model
- Easy communication via channels
- Simpler lifecycle management
- For infrastructure services like ngrok, isolation isn't critical
- Can add process-based plugins later if needed

### Plugin Discovery
**Decision**: Shared registry pattern (like modules)

**Rationale:**
- Consistent with existing module system
- Simple and type-safe
- Direct access via `plugins.Get("ngrok")`
- Can add capability-based discovery later if needed

### Exporter Triggering
**Decision**: Batched periodically + on-demand CLI

**Rationale:**
- Batching reduces I/O overhead
- On-demand provides flexibility
- Allows exporters to optimize batch operations
- Users can force export when needed

## Future Considerations

- Capability-based plugin discovery if multiple plugins provide same functionality
- Process-based plugins for better isolation if needed
- Event streaming for real-time exporters
- Plugin dependencies and startup ordering
- Health checks and automatic plugin restart
