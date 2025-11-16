# internal/events/

This package defines the core event model for DevLog. All events flowing through the system use this schema and validation logic.

## Files

### event.go
**Location:** [event.go](event.go)

Defines the Event struct, validation rules, and JSON serialization.

## Event Model

The `Event` struct represents a single development activity:

```go
type Event struct {
    Version   int                    `json:"v"`
    ID        string                 `json:"id"`
    Timestamp string                 `json:"timestamp"`
    Source    string                 `json:"source"`
    Type      string                 `json:"type"`
    Repo      string                 `json:"repo,omitempty"`
    Branch    string                 `json:"branch,omitempty"`
    Payload   map[string]interface{} `json:"payload"`
}
```

### Fields

**Version** (`v`)
- Current version: `1`
- Used for schema evolution
- Validated to be exactly `1`

**ID** (`id`)
- UUID v4 format
- Generated automatically via `uuid.New()`
- Must be unique across all events

**Timestamp** (`timestamp`)
- RFC3339 format (e.g., `2025-11-14T10:30:00Z`)
- UTC timezone
- Automatically set to current time

**Source** (`source`)
- Where the event originated
- Validated against allowed sources (see below)

**Type** (`type`)
- What kind of event occurred
- Validated against allowed types (see below)

**Repo** (`repo`)
- Optional: Path to git repository
- Used for git-related events
- Helps group events by project

**Branch** (`branch`)
- Optional: Git branch name
- Used with `repo` field
- Tracks which branch was active

**Payload** (`payload`)
- Event-specific data as JSON object
- Required (must not be nil)
- Contents vary by event type

## Event Sources

Valid sources defined as constants:

```go
const (
    SourceGit       = "git"
    SourceShell     = "shell"
    SourceWisprflow = "wisprflow"
    SourceManual    = "manual"
    SourceGitHub    = "github"
    SourceClipboard = "clipboard"
)
```

**git** - Events from git operations (commits, pushes, etc.)
**shell** - Events from shell command execution
**wisprflow** - Voice transcription events
**manual** - Events created via CLI or API manually
**github** - Events from GitHub webhooks or polling
**clipboard** - Code snippets copied to clipboard

## Event Types

Valid types defined as constants:

```go
const (
    TypeCommit        = "commit"
    TypeMerge         = "merge"
    TypePush          = "push"
    TypePull          = "pull"
    TypeFetch         = "fetch"
    TypeCheckout      = "checkout"
    TypeRebase        = "rebase"
    TypeStash         = "stash"
    TypeCommand       = "command"
    TypeNote          = "note"
    TypePRMerged      = "pr_merged"
    TypeContextSwitch = "context_switch"
    TypeTranscription = "transcription"
    TypeCopy          = "copy"
    TypeOther         = "other"
)
```

### Common Type Usage

- **commit** - Git commit created
- **command** - Shell command executed
- **merge** - Git branches merged
- **push** - Git push to remote
- **note** - Manual note or observation
- **context_switch** - Switched between projects/branches

## Validation

The `Validate()` method checks:

1. ✓ Version is exactly `1`
2. ✓ ID is present and valid UUID
3. ✓ Timestamp is present and valid RFC3339
4. ✓ Source is in allowed list
5. ✓ Type is in allowed list
6. ✓ Payload is not nil

**Example validation error:**
```go
err := event.Validate()
// Returns: "invalid source: unknown_source"
```

## Creating Events

### NewEvent(source, eventType string) *Event

Creates a new event with auto-generated ID and timestamp:

```go
event := events.NewEvent("git", "commit")
event.Repo = "/home/user/project"
event.Branch = "main"
event.Payload["hash"] = "abc123"
event.Payload["message"] = "Fix bug"
```

## JSON Serialization

### ToJSON() ([]byte, error)

Converts event to JSON bytes:

```go
data, err := event.ToJSON()
// Returns: {"v":1,"id":"...","timestamp":"...","source":"git","type":"commit","payload":{...}}
```

### FromJSON(data []byte) (*Event, error)

Parses event from JSON bytes:

```go
event, err := events.FromJSON(jsonData)
if err != nil {
    log.Fatal(err)
}
```

### PayloadJSON() (string, error)

Serializes just the payload field:

```go
payloadStr, err := event.PayloadJSON()
// Returns: {"hash":"abc123","message":"Fix bug"}
```

Used by storage layer to store payload separately.

## Example Event Payloads

### Git Commit
```json
{
  "v": 1,
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2025-11-14T10:30:00Z",
  "source": "git",
  "type": "commit",
  "repo": "/home/user/myproject",
  "branch": "feature-xyz",
  "payload": {
    "hash": "a1b2c3d4",
    "message": "Add new feature",
    "author": "John Doe"
  }
}
```

### Shell Command
```json
{
  "v": 1,
  "id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
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

### Manual Note
```json
{
  "v": 1,
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "timestamp": "2025-11-14T14:20:00Z",
  "source": "manual",
  "type": "note",
  "payload": {
    "text": "Need to refactor the authentication module",
    "tags": ["refactor", "auth"]
  }
}
```

## Testing

Tests are available in [event_test.go](event_test.go) with **93.9% coverage**.

Run tests:
```bash
go test ./internal/events/...
go test -cover ./internal/events/...
```

## Usage in Other Packages

- [../api/](../api/) - Validates incoming events via HTTP
- [../storage/](../storage/) - Persists events to SQLite
- [../session/](../session/) - Groups events into sessions
- [../../modules/git/](../../modules/git/) - Creates git events
- [../../modules/shell/](../../modules/shell/) - Creates shell events

## Dependencies

- `github.com/google/uuid` - UUID generation and validation
- `encoding/json` - JSON marshaling
- `time` - Timestamp parsing and formatting
