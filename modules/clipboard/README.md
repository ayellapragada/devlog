# modules/clipboard/

This module tracks clipboard changes automatically via polling, capturing copy events with intelligent deduplication to avoid recording the same content multiple times.

## Overview

The clipboard module monitors your system clipboard and records whenever you copy text. It's useful for capturing:
- Code snippets you're working with
- Documentation you're referencing
- Error messages you're debugging
- URLs and resources you're using

## Installation

```bash
devlog module install clipboard
```

### What Gets Installed

The clipboard module runs as a background poller within the daemon. No external hooks or scripts are installed - everything runs in-process.

### Privacy Notice

The clipboard tracker monitors all text you copy. This data is:
- Stored locally in your devlog database
- Never sent to external services
- Deduplicated to avoid excessive storage
- Subject to configurable size limits

## Configuration

Default configuration in `~/.config/devlog/config.yaml`:

```yaml
modules:
  clipboard:
    enabled: true
    poll_interval_seconds: 5     # How often to check clipboard
    max_length: 10000            # Max characters to capture
    min_length: 3                # Ignore content shorter than this
    dedup_history_size: 5        # Remember last N items for deduplication
```

### Configuration Options

- **poll_interval_seconds**: How frequently to check the clipboard in seconds (range: 1-3600, default: 5)
- **max_length**: Maximum characters to capture (longer content is truncated, range: 1-1000000, default: 10000)
- **min_length**: Ignore content shorter than this (avoids single characters, default: 3)
- **dedup_history_size**: Number of recent clipboard items to remember for deduplication (range: 1-100, default: 5)

## Captured Events

### clipboard/copy

Triggered when new text is copied to the clipboard.

**Event Type:** `clipboard/copy`

**Payload:**
```json
{
  "content": "the copied text...",
  "length": 42
}
```

## How It Works

### 1. Polling Mechanism

The clipboard module uses a polling approach:
```
Every 5 seconds (configurable):
  ↓
Read clipboard content
  ↓
Check if content is new (not in recent history)
  ↓
If new → create event
```

### 2. Deduplication

To avoid creating duplicate events when you copy the same thing multiple times:

1. Each clipboard content is hashed (SHA-256)
2. The module maintains a circular buffer of recent hashes
3. Before creating an event, it checks if the hash is in recent history
4. If found, the event is skipped
5. If not found, event is created and hash is added to history

**Example:**
```
Copy "hello"     → Event created (hash: abc123)
Copy "world"     → Event created (hash: def456)
Copy "hello"     → Skipped (hash abc123 in recent history)
Copy "test"      → Event created (hash: ghi789)
```

### 3. State Persistence

Recent hashes are persisted to disk so deduplication works across daemon restarts:
- State stored in `~/.config/devlog/state.json`
- On startup, recent hashes are loaded
- On each new copy, state is updated

## Use Cases

### Code Snippet Tracking

Track code snippets you copy while developing:
- Stack Overflow solutions
- Documentation examples
- Your own code for reuse

### Debug Workflow

Capture error messages and logs:
- Compiler errors
- Stack traces
- Log output

### Research Trail

Maintain a record of resources:
- URLs you're referencing
- Documentation you're reading
- Commands you're trying

## Performance

The clipboard module is designed to be lightweight:
- **Polling overhead**: Minimal CPU usage (< 0.1%)
- **Memory footprint**: Small (< 5MB)
- **Deduplication**: Prevents duplicate storage
- **Content limits**: Configurable max size prevents large captures

Typical overhead: < 1ms per poll

## Privacy Considerations

1. **Local Only**: All clipboard data stays on your machine
2. **Sensitive Content**: Be aware that passwords or API keys you copy will be captured
3. **Size Limits**: Configure `max_length` to limit what's stored
4. **Minimum Length**: Use `min_length` to avoid capturing single characters or short snippets
5. **Review Data**: Regularly review captured clipboard data

## Uninstallation

```bash
devlog module uninstall clipboard
```

This will:
- Disable clipboard polling
- Clean up state data
- Preserve historical clipboard events in your database

To remove historical data:
```bash
devlog module uninstall --purge clipboard
```

## Troubleshooting

### Clipboard not being tracked

**Check if module is enabled:**
```bash
devlog module status
```

**Check if daemon is running:**
```bash
devlog daemon status
```

**Test manually:**
1. Copy some text
2. Wait 5 seconds (or your configured poll interval)
3. Run `devlog status` and look for clipboard events

### Duplicate events

If you're seeing duplicate events for the same clipboard content:

1. Check `dedup_history_size` - increase it if needed
2. Verify state file exists: `~/.config/devlog/state.json`
3. Check daemon logs for state save errors

### Performance issues

If clipboard polling is causing issues:

1. Increase `poll_interval_seconds` (e.g., 10 or 30 seconds)
2. Decrease `max_length` to reduce processing time
3. Check system clipboard access (some tools interfere)

## See Also

- [Module system overview](../README.md)
- [Event model](../../internal/events/)
