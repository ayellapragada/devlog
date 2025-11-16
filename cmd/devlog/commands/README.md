# cmd/devlog/commands/

This directory contains the implementation of all CLI commands available in the `devlog` binary.

## Command Files

### config.go
**Location:** [config.go](config.go)

Commands for viewing and managing DevLog configuration.

**Subcommands:**
- `devlog config show` - Display current configuration
- `devlog config path` - Show path to config file

### daemon.go
**Location:** [daemon.go](daemon.go)

Commands for managing the DevLog daemon lifecycle.

**Subcommands:**
- `devlog daemon start` - Start the daemon in foreground
- `devlog daemon stop` - Stop a running daemon
- `devlog daemon status` - Check daemon status and connectivity

### help.go
**Location:** [help.go](help.go)

Displays comprehensive help information for all commands and subcommands.

**Usage:**
- `devlog help` - Show general help
- `devlog help <command>` - Show help for specific command
- `devlog <command> help` - Alternative help syntax

### ingest.go
**Location:** [ingest.go](ingest.go)

Manual event ingestion commands for creating events from the CLI.

**Subcommands:**
- `devlog ingest git-commit` - Ingest a git commit event
- `devlog ingest shell-command` - Ingest a shell command event
- `devlog ingest note` - Ingest a manual note

Each ingest command accepts flags for event-specific data.

### init.go
**Location:** [init.go](init.go)

Initializes DevLog by creating configuration and database files.

**Command:**
- `devlog init` - Create `~/.config/devlog/config.yaml` and `~/.local/share/devlog/events.db`

### module.go
**Location:** [module.go](module.go)

Commands for managing DevLog modules (git, shell, etc.).

**Subcommands:**
- `devlog module list` - List all available modules
- `devlog module install <name>` - Install a module
- `devlog module uninstall <name>` - Uninstall a module

### poll.go
**Location:** [poll.go](poll.go)

Commands for managing external data pollers.

**Subcommands:**
- `devlog poll start <source>` - Start polling from a source
- `devlog poll stop <source>` - Stop polling
- `devlog poll status` - Show polling status

Supported sources: `wisprflow`

### session.go
**Location:** [session.go](session.go)

Commands for creating and managing development sessions.

**Subcommands:**
- `devlog session list` - List recent sessions
- `devlog session create` - Create a session from event IDs
- `devlog session show <id>` - Show session details
- `devlog session current` - Show the active session

### status.go
**Location:** [status.go](status.go)

Display recent events and system status.

**Command:**
- `devlog status [options]` - Show recent events

**Flags:**
- `-v, --verbose` - Show detailed event information
- `-n, --number <N>` - Limit to N events (default: 10)
- `-s, --source <source>` - Filter by event source

## Command Architecture

All commands follow a similar pattern:

1. **Parse flags and arguments** from `os.Args`
2. **Load configuration** if needed
3. **Execute command logic** (API calls, database operations, etc.)
4. **Format and display output** using utilities from [../formatting/](../formatting/)
5. **Return error** for proper exit code handling

## Adding a New Command

1. Create a new `.go` file in this directory
2. Implement a function with signature: `func CommandName() error`
3. Add the command to the switch statement in [../main.go](../main.go)
4. Add help documentation in [help.go](help.go)
5. Optionally add tests in `command_test.go` files

## Testing

Test files are included for critical commands:
- [ingest_test.go](ingest_test.go) - Tests for event ingestion

Run tests:
```bash
go test ./cmd/devlog/commands/...
```
