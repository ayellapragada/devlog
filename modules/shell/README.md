# modules/shell/

This module captures shell command execution automatically by integrating with your shell's prompt system. It tracks every command you run along with exit codes, duration, and context.

## Files

### module.go
**Location:** [module.go](module.go)

Module registration, install/uninstall logic, and shell integration.

### hooks/devlog.sh
**Location:** [hooks/devlog.sh](hooks/devlog.sh)

Shell hook script that captures command execution before and after each command.

## Installation

```bash
./bin/devlog module install shell
```

### What Gets Installed

1. **Shell hook script** → `~/.local/share/devlog/hooks/devlog.sh`
   - Integrates with Bash/Zsh prompt system
   - Captures commands via `DEBUG` trap
   - Sends events to daemon

2. **RC file modification**
   - Adds source line to `~/.bashrc` or `~/.zshrc`
   - Creates backup of original RC file
   - Adds comment marking DevLog integration

### Supported Shells

- ✓ **Bash** - Uses `DEBUG` trap and `PROMPT_COMMAND`
- ✓ **Zsh** - Uses `preexec` and `precmd` hooks
- ✗ **Fish** - Not yet supported
- ✗ **Other shells** - Manual integration required

### Post-Install

Restart your shell or source the RC file:

```bash
source ~/.zshrc   # for Zsh
source ~/.bashrc  # for Bash
```

## Captured Data

Every shell command creates an event with:

### Event Fields
```json
{
  "v": 1,
  "id": "uuid",
  "timestamp": "2025-11-14T11:15:30Z",
  "source": "shell",
  "type": "command",
  "repo": "/home/user/myproject",
  "branch": "main",
  "payload": {
    "command": "npm test",
    "exit_code": 0,
    "duration": 1523,
    "workdir": "/home/user/myproject"
  }
}
```

### Payload Details

**command** - The exact command executed (string)

**exit_code** - Command exit status (integer)
- 0 = success
- Non-zero = error

**duration** - Execution time in milliseconds (integer)

**workdir** - Directory where command was run (string)

**repo** - Git repository path if inside a repo (string, optional)

**branch** - Git branch if inside a repo (string, optional)

## Command Filtering

The module supports intelligent filtering to avoid noise from trivial commands.

### Capture Modes

Configured in `~/.config/devlog/config.yaml`:

```yaml
modules:
  shell:
    enabled: true
    capture_mode: important  # or "all"
    ignore_list:
      - ls
      - cd
      - pwd
      - echo
      - cat
      - clear
```

**capture_mode: important** (default)
- Only captures "important" commands
- Filters out navigation and viewing commands
- Uses ignore list

**capture_mode: all**
- Captures every command
- Ignores the ignore_list
- Useful for debugging or detailed tracking

### Default Ignore List

```go
[]string{
    "ls", "cd", "pwd", "echo", "cat", "clear",
    "exit", "history", "which", "type", "alias",
}
```

### How Filtering Works

1. Command is executed
2. Hook captures it
3. Sends to daemon
4. Daemon checks config
5. If in ignore list → returns `{filtered: true}`
6. If not ignored → stores in database

**Note:** Filtering happens server-side, not in the shell hook. This allows for dynamic filter updates without restarting the shell.

## How It Works

### 1. Shell Integration

**Bash:**
```bash
trap 'devlog_preexec' DEBUG
PROMPT_COMMAND="devlog_postcmd"
```

**Zsh:**
```zsh
preexec() { devlog_preexec }
precmd() { devlog_postcmd }
```

### 2. Execution Flow

```
You type: npm test
         ↓
preexec hook (captures command + start time)
         ↓
npm test executes
         ↓
precmd hook (captures exit code + duration)
         ↓
Creates event JSON
         ↓
POSTs to http://127.0.0.1:8573/api/v1/ingest
```

### 3. Context Detection

The hook automatically detects:
- Current working directory
- Git repository (if inside one)
- Current branch (if in git repo)
- Command exit status
- Execution duration

## Uninstallation

```bash
./bin/devlog module uninstall shell
```

This removes:
- `~/.local/share/devlog/hooks/devlog.sh`
- Source line from `~/.bashrc` or `~/.zshrc`
- Creates backup of RC file before modification

## Configuration

### Default Configuration

```go
map[string]interface{}{
    "capture_mode": "important",
    "ignore_list": []string{
        "ls", "cd", "pwd", "echo", "cat", "clear",
        "exit", "history", "which", "type", "alias",
    },
}
```

### Custom Configuration

Edit `~/.config/devlog/config.yaml`:

```yaml
modules:
  shell:
    enabled: true
    capture_mode: all
    ignore_list:
      - ls
      - pwd
      - custom_command
```

### Validation

The module validates configuration:
- `capture_mode` must be "all" or "important"
- `ignore_list` must be array of strings

Invalid config will cause daemon startup to fail.

## Implementation Details

### Module Interface

```go
type Module struct{}

func (m *Module) Name() string {
    return "shell"
}

func (m *Module) Description() string {
    return "Capture shell commands automatically"
}

func (m *Module) Install(ctx *modules.InstallContext) error {
    // Detect shell, write hook, modify RC file
}

func (m *Module) Uninstall(ctx *modules.InstallContext) error {
    // Remove hook, clean RC file
}
```

### RC File Management

**Install:**
1. Detect shell from `$SHELL`
2. Determine RC file path
3. Check if already installed
4. Create backup (`.backup.devlog`)
5. Append source line with comment

**Uninstall:**
1. Read RC file
2. Find DevLog section
3. Create backup (`.backup.devlog.uninstall`)
4. Remove integration lines
5. Write cleaned file

### Embedded Files

Hook script is embedded:

```go
//go:embed hooks/devlog.sh
var devlogShellScript string
```

## Performance Considerations

### Minimal Overhead

The hook is designed for minimal performance impact:
- Asynchronous event sending (non-blocking)
- Quick context detection
- Server-side filtering

### Network Calls

Each command makes one HTTP POST:
- Localhost only (`127.0.0.1`)
- Fast Unix socket would be future improvement
- Currently acceptable for most workflows

### Filtering Impact

Filtered commands still make network call but:
- No database write
- Fast config check
- Returns immediately

## Troubleshooting

### Commands not being captured

**Check daemon is running:**
```bash
./bin/devlog daemon status
```

**Check hook is loaded:**
```bash
type devlog_preexec
# Should show: devlog_preexec is a shell function
```

**Check RC file:**
```bash
grep devlog ~/.zshrc  # or ~/.bashrc
# Should show: source "..."
```

### Too many commands captured

**Switch to important mode:**
```yaml
modules:
  shell:
    capture_mode: important
```

**Add to ignore list:**
```yaml
modules:
  shell:
    ignore_list:
      - your_noisy_command
```

### Hook causing errors

**Check devlog.sh exists:**
```bash
ls ~/.local/share/devlog/hooks/devlog.sh
```

**Check for syntax errors:**
```bash
bash -n ~/.local/share/devlog/hooks/devlog.sh
```

## Testing

After installation, test with:

```bash
echo "test"
./bin/devlog status
```

You should see the `echo` command (unless in ignore list) in the output.

## Privacy Considerations

The shell module captures all commands including:
- Potentially sensitive commands
- Commands with credentials (if typed)
- File paths and arguments

**Recommendations:**
1. Use environment variables for secrets
2. Add sensitive commands to ignore list
3. Review captured data regularly
4. Database is local-only (not synced anywhere)

## Dependencies

- DevLog daemon running at `http://127.0.0.1:8573`
- curl or similar for HTTP requests
- Bash or Zsh shell
- Git (optional, for repo context)

## See Also

- [Module system overview](../README.md)
- [Event model](../../internal/events/)
- [API endpoints](../../internal/api/)
- [Configuration](../../internal/config/)
