# Wispr Flow Module

This module integrates with [Wispr Flow](https://www.wisprflow.com/), a speech-to-text application, by reading from its SQLite database.

## Features

- Read-only access to Wispr Flow's SQLite database
- Polls for new speech-to-text entries (only captures entries created after installation)
- Persistent timestamp tracking to avoid duplicate captures
- Captures transcriptions with metadata (app, timestamp, duration, etc.)
- Configurable polling interval and filtering

## Installation

```bash
devlog module install wisprflow
```

## Configuration

The module can be configured in your devlog config file. Default configuration:

```yaml
modules:
  wisprflow:
    enabled: true
    config:
      poll_interval_seconds: 60
      db_path: "~/Library/Application Support/Wispr Flow/flow.sqlite"
      min_words: 1
      include_fields:
        - transcriptEntityId
        - asrText
        - formattedText
        - editedText
        - timestamp
        - status
        - app
        - url
        - duration
        - numWords
```

### Configuration Options

- `poll_interval_seconds`: How often to poll the database (default: 60)
- `db_path`: Path to the Wispr Flow SQLite database
- `min_words`: Minimum number of words to capture (default: 1)
- `include_fields`: List of fields to include from the History table

## Database Structure

The module reads from the `History` table in the Wispr Flow database. Key fields:

- `transcriptEntityId`: Unique identifier for each transcription
- `asrText`: Raw automatic speech recognition text
- `formattedText`: Formatted/cleaned up text
- `editedText`: User-edited text (if any)
- `timestamp`: When the transcription occurred
- `app`: Application where the text was dictated
- `url`: URL if applicable
- `duration`: Audio duration in seconds
- `numWords`: Number of words in the transcription

## Safety

- The module opens the database in **read-only mode** to ensure no accidental modifications
- Uses SQLite's `file:path?mode=ro` syntax for safe access
- Skips entries with missing or invalid timestamps
- Only captures new entries created after module installation (no historical data ingestion)
- Stores last poll timestamp in `~/.local/share/devlog/wisprflow_last_poll.txt`

## Usage Example

Once installed, the module will automatically poll the Wispr Flow database at the configured interval. The daemon will capture new transcriptions and store them as events.

### Programmatic Usage

You can also use the polling function directly:

```go
import (
	"devlog/modules/wisprflow"
	"time"
)

// Load the last poll timestamp
dataDir := "/path/to/.local/share/devlog"
lastPoll, err := wisprflow.LoadLastPollTime(dataDir)
if err != nil {
	// handle error
}

// Poll for new entries since last poll
dbPath := wisprflow.GetDBPath(homeDir, "")
entries, err := wisprflow.PollDatabase(dbPath, lastPoll)
if err != nil {
	// handle error
}

// Process entries
for _, entry := range entries {
	fmt.Printf("%s: %s\n", entry.Timestamp, entry.FormattedText)
}

// Save the new poll timestamp
if len(entries) > 0 {
	lastEntry := entries[len(entries)-1]
	wisprflow.SaveLastPollTime(dataDir, lastEntry.Timestamp)
}
```

## Requirements

- Wispr Flow must be installed on your system
- The SQLite database must be accessible at the configured path
- Go SQLite driver (`github.com/mattn/go-sqlite3`) is automatically included
