# modules/git/

This module captures git operations automatically by wrapping the `git` command. It intercepts git commands and sends events to DevLog after successful operations.

## Files

### module.go
**Location:** [module.go](module.go)

Module registration and install/uninstall logic.

### hooks/git-wrapper.sh
**Location:** [hooks/git-wrapper.sh](hooks/git-wrapper.sh)

Shell script that wraps the real `git` binary and captures operations.

### hooks/devlog-git-common.sh
**Location:** [hooks/devlog-git-common.sh](hooks/devlog-git-common.sh)

Shared library functions for git event capture logic.

## Installation

```bash
./bin/devlog module install git
```

### What Gets Installed

1. **Git wrapper script** → `~/.local/bin/git`
   - Intercepts all git commands
   - Calls the real git binary
   - Captures successful operations

2. **Common library** → `~/.local/bin/devlog-git-common.sh`
   - Shared functions for event creation
   - Parses git output
   - Sends events to daemon

3. **PATH modification required**
   - Add `~/.local/bin` to start of PATH
   - Must come before `/usr/bin` to intercept git

### Post-Install

Add to your shell RC file (`~/.zshrc` or `~/.bashrc`):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Then restart your shell:
```bash
source ~/.zshrc  # or ~/.bashrc
```

## Captured Events

The module captures these git operations:

### commit
Triggered after successful `git commit`

**Payload:**
```json
{
  "hash": "a1b2c3d4",
  "message": "Commit message",
  "author": "John Doe"
}
```

### push
Triggered after successful `git push`

**Payload:**
```json
{
  "remote": "origin",
  "branch": "main",
  "commits": 3
}
```

### pull
Triggered after successful `git pull`

**Payload:**
```json
{
  "remote": "origin",
  "branch": "main",
  "changes": "Fast-forward"
}
```

### merge
Triggered after successful `git merge`

**Payload:**
```json
{
  "source_branch": "feature-xyz",
  "target_branch": "main",
  "strategy": "recursive"
}
```

### rebase
Triggered after successful `git rebase`

**Payload:**
```json
{
  "onto": "main",
  "commits_applied": 5
}
```

### checkout
Triggered after `git checkout` (branch switch)

**Payload:**
```json
{
  "from_branch": "main",
  "to_branch": "feature-xyz",
  "type": "branch"
}
```

### stash
Triggered after `git stash`

**Payload:**
```json
{
  "action": "save",
  "message": "WIP: working on feature"
}
```

### fetch
Triggered after successful `git fetch`

**Payload:**
```json
{
  "remote": "origin",
  "refs_updated": 2
}
```

## How It Works

### 1. Command Interception

When you run `git commit`:
```
you type: git commit -m "message"
         ↓
~/.local/bin/git (wrapper)
         ↓
/usr/bin/git (real git)
         ↓
wrapper captures success
         ↓
sends event to devlogd
```

### 2. Event Creation

The wrapper script:
1. Executes real git command
2. Checks exit code (0 = success)
3. Parses git output for details
4. Creates event JSON
5. POSTs to `http://127.0.0.1:8573/api/v1/ingest`

### 3. Context Detection

The wrapper automatically detects:
- Current repository path
- Current branch name
- Remote names and URLs

## Uninstallation

```bash
./bin/devlog module uninstall git
```

This removes:
- `~/.local/bin/git` (wrapper script)
- `~/.local/bin/devlog-git-common.sh` (library)

**Note:** Only removes files if they match DevLog's version. If you've modified the wrapper, it will skip removal with a warning.

## Configuration

No configuration required. The module works globally for all repositories once installed.

## Implementation Details

### Module Interface

```go
type Module struct{}

func (m *Module) Name() string {
    return "git"
}

func (m *Module) Description() string {
    return "Capture git operations automatically"
}

func (m *Module) Install(ctx *modules.InstallContext) error {
    // Write git wrapper and common lib
}

func (m *Module) Uninstall(ctx *modules.InstallContext) error {
    // Remove wrapper and lib
}
```

### Embedded Files

Scripts are embedded using `go:embed`:

```go
//go:embed hooks/git-wrapper.sh
var gitWrapperScript string

//go:embed hooks/devlog-git-common.sh
var gitCommonLib string
```

This allows the binary to be self-contained with all required files.

## Troubleshooting

### Git commands not being captured

**Check PATH order:**
```bash
which git
# Should show: /Users/username/.local/bin/git
```

If it shows `/usr/bin/git`, your PATH is incorrect.

**Fix:**
```bash
export PATH="$HOME/.local/bin:$PATH"
```

### Daemon not receiving events

**Check daemon is running:**
```bash
./bin/devlog daemon status
```

**Check daemon logs:**
```bash
./bin/devlog status
```

### Wrapper conflicts

If another tool also wraps git, you may see issues. Check:
```bash
cat ~/.local/bin/git
```

Ensure it contains DevLog's wrapper code.

## Testing

After installation, test with:

```bash
cd /path/to/any/repo
git commit --allow-empty -m "Test commit"
./bin/devlog status
```

You should see the commit event in the output.

## Dependencies

- Real git binary (must be installed at `/usr/bin/git` or in PATH)
- DevLog daemon running at `http://127.0.0.1:8573`
- curl (for posting events)
- bash (for wrapper script)

## See Also

- [Module system overview](../README.md)
- [Event model](../../internal/events/)
- [API endpoints](../../internal/api/)
