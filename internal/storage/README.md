# internal/storage/

This package provides SQLite database operations for DevLog's event and session persistence layer.

## Files

### storage.go
**Location:** [storage.go](storage.go)

Core database interface with methods for event CRUD operations and session management.

### migrations.go
**Location:** [migrations.go](migrations.go)

Database schema migrations for version management and upgrades.

### session_storage.go
**Location:** [session_storage.go](session_storage.go)

Session-specific database operations including session CRUD and event linking.

## Storage Interface

The `Storage` struct wraps a SQLite database connection:

```go
type Storage struct {
    db *sql.DB
}
```

## Database Schema

### events Table
Stores all development events captured by DevLog.

```sql
CREATE TABLE events (
    id TEXT PRIMARY KEY,              -- UUID
    timestamp TEXT NOT NULL,          -- RFC3339 format
    source TEXT NOT NULL,             -- git, shell, manual, etc.
    type TEXT NOT NULL,               -- commit, command, note, etc.
    repo TEXT,                        -- Optional repository path
    branch TEXT,                      -- Optional git branch
    payload TEXT NOT NULL,            -- JSON payload
    created_at INTEGER NOT NULL       -- Unix timestamp
);
```

**Indexes:**
- `idx_timestamp` on `timestamp`
- `idx_repo` on `repo`
- `idx_source` on `source`
- `idx_created_at` on `created_at`

### sessions Table
Groups related events into development sessions.

```sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,              -- UUID
    start_time INTEGER NOT NULL,      -- Unix timestamp
    end_time INTEGER,                 -- Unix timestamp (NULL if active)
    status TEXT NOT NULL,             -- active, completed, abandoned
    start_trigger TEXT NOT NULL,      -- first_commit, manual, etc.
    end_trigger TEXT,                 -- timeout, explicit_end, etc.
    description TEXT,                 -- Optional description
    primary_repo TEXT,                -- Main repository for session
    branch TEXT,                      -- Main branch for session
    metadata TEXT NOT NULL,           -- JSON metadata
    created_at INTEGER NOT NULL       -- Unix timestamp
);
```

**Indexes:**
- `idx_session_status` on `status`
- `idx_session_start_time` on `start_time`
- `idx_session_primary_repo` on `primary_repo`

### session_events Table
Many-to-many relationship between sessions and events.

```sql
CREATE TABLE session_events (
    session_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (session_id, event_id),
    FOREIGN KEY (session_id) REFERENCES sessions(id),
    FOREIGN KEY (event_id) REFERENCES events(id)
);
```

**Indexes:**
- `idx_session_events_session` on `session_id`
- `idx_session_events_event` on `event_id`

## Key Methods

### Event Operations

**InsertEvent(event \*events.Event) error**
- Validates and inserts a new event
- Uses context with 5s timeout
- Returns error if event is invalid

**GetEvent(id string) (\*events.Event, error)**
- Retrieves a single event by UUID
- Returns error if not found

**ListEvents(limit int, source string) ([]*events.Event, error)**
- Lists recent events with optional filtering
- Orders by `created_at DESC`
- Supports source filtering (e.g., "git", "shell")

**Count() (int, error)**
- Returns total event count

### Session Operations

**InsertSession(session \*session.Session) error**
- Creates a new session record
- Links events via `session_events` table

**GetSession(id string) (\*session.Session, error)**
- Retrieves session with all linked events

**ListSessions(limit int, status string) ([]*session.Session, error)**
- Lists sessions with optional status filter
- Orders by `start_time DESC`

**UpdateSessionStatus(id string, status string, endTime \*time.Time) error**
- Updates session status and end time

## Database Features

### WAL Mode
Write-Ahead Logging is enabled for concurrent reads/writes:

```go
db.Exec("PRAGMA journal_mode=WAL")
```

This allows:
- Multiple readers simultaneously
- One writer at a time
- Better performance for DevLog's workload

### Context Timeouts
All database operations use context with timeouts:
- Event operations: 5 seconds
- List operations: 10 seconds
- Prevents hanging queries

### Migration System
Schema changes are tracked in [migrations.go](migrations.go):

```go
func RunMigrations(db *sql.DB) error {
    // Check current version
    // Apply pending migrations
    // Update version
}
```

## Initialization

### InitDB(dbPath string) error
Creates a new database with schema:

```bash
devlog init  # Creates ~/.local/share/devlog/events.db
```

**Steps:**
1. Check database doesn't exist
2. Create directory structure
3. Create SQLite file
4. Enable WAL mode
5. Execute schema SQL
6. Create indexes

### New(dbPath string) (\*Storage, error)
Opens an existing database:

**Steps:**
1. Check database exists
2. Open SQLite connection
3. Enable WAL mode
4. Run any pending migrations
5. Return Storage instance

## Testing

Tests are available in [storage_test.go](storage_test.go) with **82.8% coverage**.

Run tests:
```bash
go test ./internal/storage/...
go test -cover ./internal/storage/...
```

## Usage Example

```go
import "devlog/internal/storage"

// Open database
store, err := storage.New("/path/to/events.db")
if err != nil {
    log.Fatal(err)
}
defer store.Close()

// Insert event
event := events.NewEvent("git", "commit")
err = store.InsertEvent(event)

// List recent events
events, err := store.ListEvents(10, "")

// Get event count
count, err := store.Count()
```

## Dependencies

- [../events/](../events/) - Event model
- [../session/](../session/) - Session model
- `modernc.org/sqlite` - Pure Go SQLite driver
- `database/sql` - Standard database interface
