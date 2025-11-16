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
./bin/devlog module install git
```

The git module installs a wrapper script to `~/.local/bin/git` that intercepts git commands and sends events to the DevLog daemon after successful operations.

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
./bin/devlog module install shell
```

The shell module integrates with your shell's prompt command (Bash/Zsh) to capture every command execution.

### clipboard
**Location:** [modules/clipboard/](clipboard/)

Monitors clipboard for code snippets and development-related content.

### wisprflow
**Location:** [modules/wisprflow/](wisprflow/)

Integrates with Wisprflow for voice transcription capture during development sessions.

## Module Interface

All modules implement the following interface defined in [internal/modules/](../internal/modules/):

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

## Module Registration

Modules register themselves on package initialization using:

```go
func init() {
    modules.Register(&Module{})
}
```

This happens automatically when the module package is imported in [cmd/devlog/main.go](../cmd/devlog/main.go).

## Creating a New Module

1. Create a new directory under `modules/`
2. Implement the Module interface
3. Add `init()` function to register the module
4. Import the module in `cmd/devlog/main.go` with `_ "devlog/modules/yourmodule"`
5. Add any hook scripts or templates as embedded files using `//go:embed`

## Configuration

Module-specific configuration is stored in `~/.config/devlog/config.yaml` under the `modules` key:

```yaml
modules:
  git:
    enabled: true
  shell:
    enabled: true
    capture_mode: important
    ignore_list:
      - ls
      - cd
      - pwd
```

Each module validates its own configuration section via the `ValidateConfig` method.
