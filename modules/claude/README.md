# modules/claude/

This module captures Claude Code conversation history and activity by polling the Claude Code projects directory. It extracts conversations, file operations, and shell commands to provide context about your AI-assisted development work.

## Overview

The claude module monitors your Claude Code workspace and records:
- Conversation exchanges between you and Claude
- File operations performed by Claude (reads, edits, writes)
- Shell commands executed by Claude
- Work session boundaries

This provides a rich record of your AI-assisted development workflow that integrates seamlessly with other devlog events.

## Installation

```bash
devlog module install claude
```

### What Gets Installed

The claude module runs as a background poller within the daemon. No external hooks or scripts are installed - everything runs in-process.

**Installation checks:**
- Verifies Claude Code projects directory exists at `~/.claude/projects`
- Counts available project directories
- Confirms read access to conversation files

### Prerequisites

- Claude Code installed and configured
- At least one project directory in `~/.claude/projects`

## Configuration

Default configuration in `~/.config/devlog/config.yaml`:

```yaml
modules:
  claude:
    enabled: true
    poll_interval_seconds: 60
    projects_dir: ~/.claude/projects
    extract_commands: true
    extract_file_edits: true
    min_message_length: 10
```

### Configuration Options

- **poll_interval_seconds**: How frequently to check for new conversations in seconds (range: 5-600, default: 60)
- **projects_dir**: Location of Claude Code projects (default: `~/.claude/projects`)
- **extract_commands**: Whether to create events for shell commands Claude runs (default: true)
- **extract_file_edits**: Whether to create events for file edits Claude makes (default: true)
- **min_message_length**: Minimum message length to capture (default: 10)

## Captured Events

### claude/conversation

Main conversation event capturing user-Claude exchanges.

**Event Type:** `claude/conversation`

**Payload:**
```json
{
  "session_id": "abc123",
  "user_message": "Help me implement a feature...",
  "claude_reply": "I'll help you implement that...",
  "summary": "Help me implement a feature... (truncated to 200 chars)",
  "command_count": 3,
  "edit_count": 5,
  "read_count": 2
}
```

### claude/command

Shell commands executed by Claude during a conversation.

**Event Type:** `claude/command` (only when `extract_commands: true`)

**Payload:**
```json
{
  "session_id": "abc123",
  "command": "go test ./...",
  "description": "Run tests",
  "stdout": "ok      devlog/modules/claude  0.123s",
  "stderr": ""
}
```

### claude/file_edit

File modifications made by Claude during a conversation.

**Event Type:** `claude/file_edit` (only when `extract_file_edits: true`)

**Payload:**
```json
{
  "session_id": "abc123",
  "file_path": "modules/claude/poller.go",
  "old_string": "func OldImplementation()... (truncated to 500 chars)",
  "new_string": "func NewImplementation()... (truncated to 500 chars)"
}
```

## How It Works

### 1. Polling Mechanism

The claude module uses a polling approach to discover new conversations:

```
Every 60 seconds (configurable):
  ↓
Scan ~/.claude/projects/* directories
  ↓
Read .jsonl conversation files
  ↓
Parse for new entries since last poll
  ↓
Extract events from conversations
  ↓
Store in devlog database
```

### 2. Conversation Parsing

Each Claude Code conversation is stored in JSONL format. The module:

1. Reads each `.jsonl` file in project directories
2. Parses conversation entries with timestamps
3. Filters for entries newer than last poll time
4. Extracts user messages, Claude replies, and tool uses
5. Creates appropriate events based on configuration

### 3. State Management

The module tracks polling state to avoid duplicate events:
- Last poll timestamp stored in `~/.config/devlog/state.json`
- Only conversations newer than last poll are processed
- State persists across daemon restarts
- Initial poll looks back 24 hours

### 4. Event Extraction

From each conversation, the module can extract:

**Conversation Summary:**
- User's initial message
- Claude's response
- Counts of commands/edits/reads performed

**Commands:**
- Shell command text
- Description of what it does
- Standard output and error
- Linked to parent conversation via session_id

**File Edits:**
- Path to edited file
- Old content (truncated to 500 chars)
- New content (truncated to 500 chars)
- Linked to parent conversation via session_id

## Use Cases

### Development History Tracking

Track your AI-assisted development workflow:
- What questions you asked Claude
- What tasks Claude helped with
- How Claude modified your codebase
- Commands Claude ran for testing/validation

### Context Recovery

Return to work after interruptions:
- Review recent Claude conversations
- See what files were modified
- Understand what commands were run
- Reconstruct your development context

### AI Usage Analysis

Understand how you use AI assistance:
- Frequency of Claude interactions
- Types of tasks you ask Claude to help with
- Patterns in file modifications
- Command execution patterns

### Team Insights

For teams using Claude Code:
- How AI tools are being leveraged
- Common AI-assisted workflows
- Areas where AI assistance is most valuable

## Performance

The claude module is designed to be efficient:
- **Polling overhead**: Minimal CPU usage
- **Memory footprint**: Small, only processes new entries
- **State tracking**: Incremental updates, no full scans
- **Content limits**: Truncates large strings to manageable sizes

Typical overhead: < 10ms per poll (depends on conversation activity)

## Privacy Considerations

1. **Local Only**: Conversation data stays on your machine
2. **Claude Context**: Your Claude conversations may contain sensitive code or discussions
3. **File Contents**: File edit events capture code snippets
4. **Command Output**: Shell output may contain sensitive information
5. **Review Data**: Regularly review captured Claude events

The module only reads data already stored locally by Claude Code - it doesn't send anything externally.

## Uninstallation

```bash
devlog module uninstall claude
```

This will:
- Disable Claude Code polling
- Clean up state data
- Preserve historical Claude events in your database

To remove historical data:
```bash
devlog module uninstall --purge claude
```

## Troubleshooting

### No events being captured

**Check if Claude Code directory exists:**
```bash
ls ~/.claude/projects
```

**Check if module is enabled:**
```bash
devlog module status
```

**Check if daemon is running:**
```bash
devlog daemon status
```

**Verify project directories:**
```bash
ls -la ~/.claude/projects/
```

### Missing conversations

**Check conversation files:**
```bash
find ~/.claude/projects -name "*.jsonl" -ls
```

**Check poll interval** - may need to wait for next poll cycle

**Check state file** - may need to reset polling state:
```bash
# View current state
cat ~/.config/devlog/state.json

# The module will automatically pick up from last saved timestamp
```

### Too many events

If you're getting too many events:

1. Increase `poll_interval_seconds` (poll less frequently)
2. Disable command extraction: `extract_commands: false`
3. Disable file edit extraction: `extract_file_edits: false`
4. Increase `min_message_length` to filter short messages

### Performance issues

If Claude polling is causing issues:

1. Increase `poll_interval_seconds` to reduce frequency
2. Check for very large JSONL files (may need cleanup)
3. Review daemon logs for parsing errors

### JSONL Parsing

The module parses Claude Code's JSONL conversation format:
- Each line is a JSON object representing a message or tool use
- Timestamps determine conversation chronology
- Tool uses indicate commands run and files edited
- Session IDs link related events together

## Dependencies

- DevLog daemon running
- Claude Code installed with accessible projects directory
- Read access to `~/.claude/projects`

## See Also

- [Module system overview](../README.md)
- [Event model](../../internal/events/)
- [State management](../../internal/state/)
