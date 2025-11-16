# internal/api/

This package implements the HTTP API server for DevLog's daemon. It provides REST endpoints for event ingestion, status checks, and session management.

## Files

### handlers.go
**Location:** [handlers.go](handlers.go)

Contains all HTTP request handlers and the API server implementation.

## API Server

The `Server` struct coordinates between storage, session management, and configuration:

```go
type Server struct {
    storage        *storage.Storage
    sessionManager *session.Manager
    config         *config.Config
    logger         *logger.Logger
    startTime      time.Time
}
```

## Endpoints

### POST /api/v1/ingest
Ingest a new event into DevLog.

**Request Body:**
```json
{
  "v": 1,
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2025-11-14T10:30:00Z",
  "source": "git",
  "type": "commit",
  "repo": "/path/to/repo",
  "branch": "main",
  "payload": {
    "hash": "abc123",
    "message": "Fix bug"
  }
}
```

**Response:**
```json
{
  "ok": true,
  "event_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Features:**
- Validates event schema before storage
- Filters shell commands based on config (ignore list)
- Returns `filtered: true` for ignored commands
- Logs all ingestion attempts

**Implementation:** `IngestHandler()` in [handlers.go:36](handlers.go#L36)

### GET /api/v1/status
Check daemon status and get event count.

**Response:**
```json
{
  "running": true,
  "event_count": 1523,
  "uptime_seconds": 3600
}
```

**Implementation:** `StatusHandler()` in [handlers.go:95](handlers.go#L95)

### GET /api/v1/sessions
List recent development sessions.

**Response:**
```json
{
  "sessions": [
    {
      "id": "session-uuid",
      "start_time": "2025-11-14T10:00:00Z",
      "end_time": "2025-11-14T12:30:00Z",
      "duration": "2h 30m 0s",
      "description": "Working on API endpoints",
      "event_count": 45,
      "status": "completed"
    }
  ],
  "count": 1
}
```

**Implementation:** `handleListSessions()` in [handlers.go:145](handlers.go#L145)

### POST /api/v1/sessions
Create a new session from specific event IDs.

**Request Body:**
```json
{
  "event_ids": ["event-uuid-1", "event-uuid-2"],
  "description": "Bug fix session"
}
```

**Response:**
```json
{
  "ok": true,
  "session_id": "new-session-uuid",
  "description": "Bug fix session",
  "event_count": 2
}
```

**Implementation:** `handleCreateSession()` in [handlers.go:173](handlers.go#L173)

## Error Handling

All error responses follow this format:

```json
{
  "ok": false,
  "error": "Error message here"
}
```

Common HTTP status codes:
- `200` - Success
- `400` - Bad request (validation error)
- `405` - Method not allowed
- `500` - Internal server error
- `503` - Service unavailable (session management disabled)

## Request Flow

```
HTTP Request
    ↓
IngestHandler
    ↓
Event Validation (events.Validate)
    ↓
Command Filtering (config.ShouldCaptureCommand)
    ↓
Storage (storage.InsertEvent)
    ↓
JSON Response
```

## Testing

Tests are available in [handlers_test.go](handlers_test.go) with **83.8% coverage**.

Run tests:
```bash
go test ./internal/api/...
go test -cover ./internal/api/...
```

## Usage Example

```bash
# Ingest an event
curl -X POST http://127.0.0.1:8573/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '{"v":1,"id":"uuid","timestamp":"2025-11-14T10:30:00Z","source":"git","type":"commit","payload":{}}'

# Check status
curl http://127.0.0.1:8573/api/v1/status

# List sessions
curl http://127.0.0.1:8573/api/v1/sessions
```

## Dependencies

- [../events/](../events/) - Event model and validation
- [../storage/](../storage/) - Database operations
- [../session/](../session/) - Session management
- [../config/](../config/) - Configuration
- [../logger/](../logger/) - Structured logging
