# DevLog

A local development journaling system that automatically captures development activity and stores it for future summarization into Obsidian notes.

## Features (v0.1-v0.2)

- **Background Daemon**: Runs locally, accepts events via HTTP API
- **SQLite Storage**: Persistent event storage with WAL mode for concurrent writes
- **Git Hook Integration**: Automatically captures commits via post-commit hooks
- **Shell Hook Integration**: Automatically captures shell commands with exit codes and duration
- **CLI**: Manage daemon, ingest events, and view recent activity
- **Type-Safe Events**: Validated event model with multiple source types
- **Smart Filtering**: Configurable command filtering to capture only important events

## Installation

### Build from Source

```bash
go build -o bin/devlog ./cmd/devlog
go build -o bin/devlogd ./cmd/devlogd
```

### Initialize

```bash
./bin/devlog init
```

This creates:
- Config file: `~/.config/devlog/config.yaml`
- Database: `~/.local/share/devlog/events.db`

Edit the config to set your Obsidian vault path.

## Usage

### Start the Daemon

```bash
./bin/devlog daemon start
```

The daemon runs in the foreground. Use Ctrl+C to stop gracefully, or:

```bash
./bin/devlog daemon stop
```

### Check Status

```bash
./bin/devlog daemon status
./bin/devlog status  # Shows recent events
```

### Install Modules

DevLog uses a modular architecture. Install modules to enable specific event capture:

```bash
# List available modules
./bin/devlog module list

# Install git module (captures commits globally)
./bin/devlog module install git

# Install shell module (captures shell commands)
./bin/devlog module install shell
```

After installing the git module, every commit in **any repository** will be automatically captured by devlog.

After installing the shell module:
- All shell commands are captured (filtered based on your config)
- Exit codes and command duration are tracked
- Commands are linked to git repos when run inside them
- Restart your shell or run `source ~/.bashrc` (or `~/.zshrc`) to activate

Configure module behavior in `~/.config/devlog/config.yaml`:

```yaml
modules:
  git:
    enabled: true
  shell:
    enabled: true
    capture_mode: important  # or "all"
    ignore_list:
      - ls
      - cd
      - pwd
      - echo
```

### Uninstall Modules

To remove a module:

```bash
# Uninstall git module (removes hooks)
./bin/devlog module uninstall git

# Uninstall shell module (removes hooks and cleans RC files)
./bin/devlog module uninstall shell
```

### Manual Event Ingestion

**Git commit:**
```bash
./bin/devlog ingest git-commit \
  --repo=/path/to/repo \
  --branch=main \
  --hash=abc123 \
  --message="Fix bug" \
  --author="Your Name"
```

**Shell command:**
```bash
./bin/devlog ingest shell-command \
  --command="npm test" \
  --exit-code=0 \
  --workdir=/path/to/project \
  --duration=1523
```

### Direct API Access

The daemon exposes a local HTTP API at `http://127.0.0.1:8573`.

**Ingest an event:**

```bash
curl -X POST http://127.0.0.1:8573/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "v": 1,
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "timestamp": "2025-11-14T10:30:00Z",
    "source": "manual",
    "type": "note",
    "payload": {"text": "My note"}
  }'
```

**Check status:**

```bash
curl http://127.0.0.1:8573/api/v1/status
```

## Architecture

```
devlog/
├── cmd/
│   ├── devlog/       # CLI binary
│   └── devlogd/      # Daemon binary
├── internal/
│   ├── config/       # Config loading
│   ├── events/       # Event model
│   ├── storage/      # SQLite operations
│   ├── api/          # HTTP handlers
│   └── daemon/       # Lifecycle management
├── hooks/            # Git hook templates
└── bin/              # Compiled binaries
```

## Event Model

All events follow this schema:

```json
{
  "v": 1,
  "id": "uuid",
  "timestamp": "2025-11-14T10:30:00Z",
  "source": "git|shell|wisprflow|manual|github",
  "type": "commit|merge|command|note|pr_merged|context_switch|other",
  "repo": "/path/to/repo",
  "branch": "main",
  "payload": {
    "hash": "abc123",
    "message": "Commit message",
    "author": "Author Name"
  }
}
```

## Testing

Run tests:

```bash
go test ./...
go test -cover ./...
```

Test coverage:
- API handlers: 83.8%
- Events: 93.9%
- Storage: 82.8%

## Roadmap

- [x] v0.1 - Basic daemon, HTTP API, SQLite, git hooks
- [x] v0.2 - Shell hooks
- [ ] v0.3 - Session grouping
- [ ] v0.4 - LLM summarization, Obsidian writing
- [ ] v0.5 - Daily summaries
- [ ] v1.0 - TUI, polish

## Configuration

Edit `~/.config/devlog/config.yaml`:

```yaml
obsidian_path: ~/Library/Mobile Documents/iCloud~md~obsidian/Documents/Main/Periodic
http:
  port: 8573
shell:
  enabled: true
  capture_mode: important  # "important" or "all"
  ignore_list:
    - ls
    - cd
    - pwd
    - echo
    - cat
    - clear
```

## License

MIT
