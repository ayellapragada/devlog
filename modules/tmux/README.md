# modules/tmux/

This module captures tmux session, window, and pane events automatically using tmux's native hook system. It provides context awareness for terminal multiplexer usage and tracks context switches between sessions.

## Files

### module.go
**Location:** [module.go](module.go)

Module registration and install/uninstall logic.

### hooks/tmux-hooks.conf
**Location:** [hooks/tmux-hooks.conf](hooks/tmux-hooks.conf)

Tmux configuration file that sets up hooks for all tracked events.

## Installation

```bash
./bin/devlog module install tmux
```

### What Gets Installed

1. **Tmux hooks config** → `~/.config/devlog/tmux-hooks.conf`
   - Tmux hook definitions for all events
   - Uses tmux's `set-hook` to capture events
   - Calls `devlog ingest` for each event

2. **Automatic tmux.conf update**
   - Adds `source-file` line to your `~/.tmux.conf`
   - Creates backup before modification
   - Automatically reloads tmux config if tmux is running

### Post-Install

The installation is fully automated! The module will:
- Create the hooks config file
- Add the source line to `~/.tmux.conf`
  - If you use TPM (tmux plugin manager), it will insert above the TPM initialization
  - Otherwise, it will append to the end of the file
- Reload your tmux configuration automatically

If tmux is not running or reload fails, you can manually reload:
```bash
tmux source-file ~/.tmux.conf
```

## Captured Events

The module captures these tmux events:

### Session Events

#### session create
Triggered after `tmux new-session` or similar

**Event Type:** `tmux_session`

**Payload:**
```json
{
  "session": "dev",
  "action": "create"
}
```

#### session close
Triggered when a session ends

**Event Type:** `tmux_session`

**Payload:**
```json
{
  "session": "dev",
  "action": "close"
}
```

#### session rename
Triggered after renaming a session

**Event Type:** `tmux_session`

**Payload:**
```json
{
  "session": "new-name",
  "action": "rename"
}
```

### Client Events

#### attach
Triggered when attaching to a session

**Event Type:** `tmux_attach`

**Payload:**
```json
{
  "session": "dev",
  "client": "client0"
}
```

#### detach
Triggered when detaching from a session

**Event Type:** `tmux_detach`

**Payload:**
```json
{
  "session": "dev",
  "client": "client0"
}
```

#### session-switch
Triggered when switching between sessions

**Event Type:** `context_switch`

**Payload:**
```json
{
  "session": "work",
  "window": "editor"
}
```

### Window Events

#### window create
Triggered after creating a new window

**Event Type:** `tmux_window`

**Payload:**
```json
{
  "session": "dev",
  "window": "editor",
  "window_id": "1",
  "action": "create"
}
```

### Pane Events

#### pane split
Triggered after splitting a window into panes

**Event Type:** `tmux_pane`

**Payload:**
```json
{
  "session": "dev",
  "window": "editor",
  "pane": "2",
  "action": "split"
}
```

## How It Works

### 1. Tmux Native Hooks

Tmux has a built-in hook system that triggers on events:

```tmux
set-hook -g after-new-session 'run-shell "devlog ingest tmux-event ..."'
```

When you create a session:
```
you type: tmux new-session -s dev
         ↓
tmux creates session
         ↓
after-new-session hook fires
         ↓
runs: devlog ingest tmux-event
         ↓
sends event to devlogd
```

### 2. Event Capture Flow

The hook system:
1. Tmux detects event (e.g., window switch)
2. Fires corresponding hook
3. Hook runs `devlog ingest tmux-event` with context
4. CLI creates event JSON with tmux variables
5. POSTs to `http://127.0.0.1:8573/api/v1/ingest`

### 3. Tmux Variables

Hooks use tmux's format variables:
- `#S` - Session name
- `#W` - Window name
- `#I` - Window index
- `#{pane_id}` - Pane ID
- `#{pane_index}` - Pane index
- `#{client_name}` - Client name

These are expanded by tmux before running the command.

## Uninstallation

```bash
./bin/devlog module uninstall tmux
```

The uninstallation is fully automated! It will:
- Remove `~/.config/devlog/tmux-hooks.conf`
- Remove the source line from `~/.tmux.conf`
- Create a backup before modification
- Automatically reload tmux configuration

If tmux is not running or reload fails, manually reload:
```bash
tmux source-file ~/.tmux.conf
```

## Use Cases

### Context Switch Detection

Track when you switch between different work contexts:
- Moving between sessions for different projects

### Session Analysis

Understand your tmux usage patterns:
- Which sessions you work in most
- How long you stay in each session
- When you create/destroy sessions

### Cognitive Load Tracking

Detect potential high cognitive load:
- Frequent session changes
- Many pane splits

## Implementation Details

### Module Interface

```go
type Module struct{}

func (m *Module) Name() string {
    return "tmux"
}

func (m *Module) Description() string {
    return "Capture tmux session, window, and pane events"
}

func (m *Module) Install(ctx *modules.InstallContext) error {
    // Write tmux hooks config
}

func (m *Module) Uninstall(ctx *modules.InstallContext) error {
    // Remove hooks config
}
```

### Embedded Files

Hook config is embedded using `go:embed`:

```go
//go:embed hooks/tmux-hooks.conf
var tmuxHooksConfig string
```

This allows the binary to be self-contained with all required files.

### CLI Ingest Command

The `devlog ingest tmux-event` command accepts flags:

```bash
devlog ingest tmux-event \
  --type=session \
  --action=create \
  --session=dev
```

## Troubleshooting

### Events not being captured

**Check if hooks are loaded:**
```bash
tmux show-hooks -g | grep devlog
```

If empty, your hooks config isn't sourced.

**Fix:**
1. Verify `~/.config/devlog/tmux-hooks.conf` exists
2. Check `~/.tmux.conf` has the source line
3. Reload: `tmux source-file ~/.tmux.conf`

### Daemon not receiving events

**Check daemon is running:**
```bash
./bin/devlog daemon status
```

**Test manually:**
```bash
devlog ingest tmux-event --type=session --action=test --session=test
./bin/devlog status
```

You should see the test event.

### Hooks firing but no events

**Check devlog binary is in PATH:**
```bash
which devlog
```

The hooks call `devlog` directly, so it must be accessible.

**Fix:**
```bash
export PATH="$HOME/.local/bin:$PATH"  # or wherever devlog is installed
```

## Testing

After installation, test with:

```bash
# Test session creation
tmux new-session -d -s test-session
./bin/devlog status

# Test window creation
tmux new-window -t test-session
./bin/devlog status

# Test pane split
tmux split-window -t test-session
./bin/devlog status

# Cleanup
tmux kill-session -t test-session
```

You should see tmux events in the status output.

## Performance

The tmux module is extremely lightweight:
- **Zero polling** - event-driven via hooks
- **Async execution** - hooks run in background
- **No overhead** - only fires on actual tmux events
- **Fast ingestion** - events queue if daemon offline

Typical overhead per event: <5ms

## Dependencies

- tmux (version 2.0+, hooks support)
- DevLog daemon running at `http://127.0.0.1:8573`
- devlog binary in PATH

## See Also

- [Module system overview](../README.md)
- [Event model](../../internal/events/)
- [ROADMAP](../../ROADMAP.md) - V0.5 Context Awareness
