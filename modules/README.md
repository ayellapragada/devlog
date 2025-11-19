# modules/

This directory contains pluggable modules that capture development events from various sources. Each module implements the Module interface and registers itself for installation via the CLI.

## Available Modules

### git
**Location:** [modules/git/](git/)

Captures git operations automatically by installing a git command wrapper.

**Events Captured:**
- Commits
- Pushes
- Pulls
- Merges
- Rebases
- Checkouts
- Stashes
- Fetches

**Installation:**
```bash
devlog module install git
```

The git module installs a wrapper script to `~/.local/bin/git` that intercepts git commands and sends events to the DevLog daemon after successful operations.

### kubectl
**Location:** [modules/kubectl/](kubectl/)

Captures kubectl operations automatically by installing a kubectl command wrapper.

**Events Captured:**
- apply
- create
- delete
- get
- describe
- edit
- patch
- logs
- exec
- debug

**Installation:**
```bash
devlog module install kubectl
```

The kubectl module installs wrapper scripts to `~/.local/bin/kubectl` and `~/.local/bin/k` that intercept kubectl commands and send events to the DevLog daemon after successful operations.

### shell
**Location:** [modules/shell/](shell/)

Captures shell commands with exit codes, duration, and working directory context.

**Events Captured:**
- Shell commands (filtered based on configuration)
- Exit codes
- Execution duration
- Working directory
- Git repository context (when applicable)

**Installation:**
```bash
devlog module install shell
```

The shell module integrates with your shell's prompt command (Bash/Zsh) to capture every command execution.

### clipboard
**Location:** [modules/clipboard/](clipboard/)

Monitors clipboard for code snippets and development-related content.

**Implementation:** Pollable module (uses polling)

**Configuration:**
```yaml
modules:
  clipboard:
    enabled: true
    poll_interval: "3s"
    max_length: 10000
    min_length: 1
    dedup_history_size: 5
```

### tmux
**Location:** [modules/tmux/](tmux/)

Captures tmux session, window, and pane events using tmux's native hook system.

**Events Captured:**
- Session creation, closure, and switching
- Client attach/detach events
- Window creation and switching
- Pane creation and selection
- Focus in/out events

**Installation:**
```bash
devlog module install tmux
```

The tmux module uses tmux's built-in hooks to capture events with zero polling overhead. Installation is fully automated - it will update your `~/.tmux.conf` and reload the configuration automatically.

### wisprflow
**Location:** [modules/wisprflow/](wisprflow/)

Integrates with Wisprflow for voice transcription capture during development sessions.

**Implementation:** Pollable module (uses polling)

**Configuration:**
```yaml
modules:
  wisprflow:
    enabled: false
    poll_interval_seconds: 60
    min_words: 0
    db_path: ""  # Auto-detected if empty
```

### claude
**Location:** [modules/claude/](claude/)

Captures Claude Code conversation history and activity by monitoring the Claude Code projects directory.

**Events Captured:**
- Conversation exchanges (user messages and Claude replies)
- Shell commands executed by Claude
- File operations performed by Claude (edits, reads)
- Work session boundaries

**Implementation:** Pollable module (uses polling)

**Configuration:**
```yaml
modules:
  claude:
    enabled: true
    poll_interval_seconds: 30
    projects_dir: ~/.claude/projects
    extract_commands: true
    extract_file_edits: true
    min_message_length: 10
```

**Prerequisites:** Claude Code installed with accessible projects directory at `~/.claude/projects`

## Module Interface

All modules implement the following interface defined in [internal/modules/](../internal/modules/):

```go
type Module interface {
    Name() string
    Description() string
    Install(ctx *common.InstallContext) error
    Uninstall(ctx *common.InstallContext) error
    DefaultConfig() interface{}
    ValidateConfig(config interface{}) error
}
```

### Optional Interfaces

Modules that need periodic polling (e.g., clipboard, wisprflow) can optionally implement:

```go
type Pollable interface {
    CreatePoller(config map[string]interface{}, dataDir string) (poller.Poller, error)
}
```

**Hook-only modules** (git, shell, tmux):
- Don't implement `Pollable`
- Events captured via hooks/integrations
- Zero polling overhead

**Pollable modules** (clipboard, wisprflow):
- Implement `Pollable` interface
- Return a configured `poller.Poller` from `CreatePoller()`
- Daemon automatically creates and manages the poller
- Can be restarted without daemon restart (see [internal/daemon/modules.go](../internal/daemon/modules.go))

## Module Registration

Modules register themselves on package initialization using:

```go
func init() {
    modules.Register(&Module{})
}
```

This happens automatically when the module package is imported in [cmd/devlog/main.go](../cmd/devlog/main.go).

## Creating a New Module

### Basic Steps

1. Create a new directory under `modules/`
2. Implement the `Module` interface from [internal/modules/module.go](../internal/modules/module.go)
3. Optionally implement `Pollable` interface if your module needs polling
4. Create a `formatter.go` file that implements event formatting (see Formatting section below)
5. Add `init()` function to register the module: `modules.Register(&YourModule{})`
6. Import the module in `cmd/devlog/main.go` with `_ "devlog/modules/yourmodule"`
7. Import the module in `cmd/devlog/formatting/events.go` for formatter registration
8. Add any hook scripts or templates as embedded files using `//go:embed`
9. Use standardized error wrappers from [internal/errors](../internal/errors/)

### Hook-Based Module Example

See [modules/git/](git/) or [modules/shell/](shell/) for complete examples.

### Pollable Module Example

See [modules/clipboard/](clipboard/) or [modules/wisprflow/](wisprflow/) for complete examples.

Key points for pollable modules:
- Implement both `Module` and `Pollable` interfaces
- `CreatePoller()` receives validated config and data directory
- Return a configured `poller.Poller` that implements:
  - `Name() string`
  - `PollInterval() time.Duration`
  - `Poll(ctx context.Context) ([]*events.Event, error)`
- Daemon handles poller lifecycle automatically

### Event Formatting

Each module should define how its events are displayed by creating a `formatter.go` file:

```go
package yourmodule

import (
    "devlog/internal/events"
    "devlog/internal/formatting"
)

type YourFormatter struct{}

func init() {
    formatting.Register("yourmodule", &YourFormatter{})
}

func (f *YourFormatter) Format(event *events.Event) string {
    // Format event based on event.Type
    switch event.Type {
    case "your_event_type":
        return formatYourEvent(event)
    default:
        return fmt.Sprintf("yourmodule/%s", event.Type)
    }
}
```

**Key points:**
- Implement the `Formatter` interface from [internal/formatting/formatter.go](../internal/formatting/formatter.go)
- Register your formatter using `formatting.Register()` in an `init()` function
- The formatter receives events where `event.Source` matches your module name
- Use helper functions like `formatting.FormatDurationMs()` and `formatting.TruncateToFirstLine()` for common formatting
- Handle multiple event types if your module produces different events
- Return a concise, readable string representation of the event

**Examples:**
- [modules/git/formatter.go](git/formatter.go) - Multiple event types with metadata
- [modules/shell/formatter.go](shell/formatter.go) - Command formatting with exit codes and duration
- [modules/wisprflow/formatter.go](wisprflow/formatter.go) - Text formatting with truncation
- [modules/claude/formatter.go](claude/formatter.go) - Complex formatting with metadata arrays

## Configuration

Module-specific configuration is stored in `~/.config/devlog/config.yaml` under the `modules` key:

```yaml
modules:
  git:
    enabled: true
  shell:
    enabled: true
    ignore_list:
      - ls
      - cd
      - pwd
```

Each module validates its own configuration section via the `ValidateConfig` method.
