#!/bin/bash
# =============================================================================
# Devlog Historical Summarizer
# =============================================================================
#
# Generates summaries for historical time periods to show what summary logs
# would have looked like if the summarizer was running at that time.
#
# USAGE:
#   ./tools/historical_summarizer.sh [OPTIONS]
#
# EXAMPLES:
#   # Generate summaries for last 24 hours with 15-minute intervals
#   ./tools/historical_summarizer.sh
#
#   # Custom time range and intervals
#   ./tools/historical_summarizer.sh --hours 48 --interval 30
#
#   # Use specific model
#   ./tools/historical_summarizer.sh --model qwen2.5:14b
#
#   # Generate for a specific date range
#   ./tools/historical_summarizer.sh --start "2025-11-19 09:00" --end "2025-11-19 17:00"
#
# OPTIONS:
#   -h, --help              Show this help
#   -m, --model MODEL       Model to use (default: qwen2.5:14b)
#   --hours HOURS           Hours to look back (default: 24)
#   --interval MINS         Interval in minutes (default: 30)
#   --context MINS          Context window in minutes (default: 60)
#   --start TIMESTAMP       Start time (format: "YYYY-MM-DD HH:MM")
#   --end TIMESTAMP         End time (format: "YYYY-MM-DD HH:MM")
#   -d, --db-path PATH      Database path
#   -o, --output FILE       Output file path
#   --ollama-url URL        Ollama server URL
#
# OUTPUT:
#   Generates a markdown file with summaries for each time interval,
#   showing what the summary logs would have looked like.
#
# REQUIREMENTS:
#   - ollama (running: ollama serve)
#   - sqlite3, jq, python3, bc
#   - Database with events at ~/.local/share/devlog/events.db
#
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

OLLAMA_BASE_URL="${OLLAMA_BASE_URL:-http://localhost:11434}"
DB_PATH="${HOME}/.local/share/devlog/events.db"
MODEL="qwen2.5:14b"
HOURS_BACK=24
INTERVAL_MINUTES=30
CONTEXT_MINUTES=60
START_TIME=""
END_TIME=""
OUTPUT_FILE=""

check_ollama() {
    log_info "Checking Ollama availability..."
    if ! curl -s "${OLLAMA_BASE_URL}/api/tags" > /dev/null; then
        log_error "Ollama is not running at ${OLLAMA_BASE_URL}"
        log_info "Start Ollama with: ollama serve"
        exit 1
    fi
    log_success "Ollama is running"
}

check_database() {
    log_info "Checking database..."
    if [ ! -f "$DB_PATH" ]; then
        log_error "Database not found at $DB_PATH"
        log_info "Run 'devlog --init' to create the database"
        exit 1
    fi

    local count
    count=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM events;" 2>/dev/null || echo "0")
    log_success "Found database with ${count} events"

    if [ "$count" -lt 1 ]; then
        log_error "Database has no events. Cannot generate summaries."
        exit 1
    fi
}

pull_model() {
    local model=$1
    log_info "Checking if model ${model} is available..."

    if curl -s "${OLLAMA_BASE_URL}/api/tags" | jq -e ".models[] | select(.name == \"${model}\")" > /dev/null 2>&1; then
        log_success "Model ${model} is already available"
        return 0
    fi

    log_info "Pulling model ${model}..."
    if ollama pull "$model" > /dev/null 2>&1; then
        log_success "Successfully pulled ${model}"
        return 0
    else
        log_error "Failed to pull ${model}"
        return 1
    fi
}

fetch_events_in_range() {
    local start_epoch=$1
    local end_epoch=$2
    local output_file=$3

    sqlite3 "$DB_PATH" <<EOF > "$output_file"
SELECT json_object(
    'id', id,
    'timestamp', datetime(timestamp, 'unixepoch'),
    'source', source,
    'type', type,
    'repo', repo,
    'branch', branch,
    'payload', payload
)
FROM events
WHERE timestamp >= $start_epoch AND timestamp < $end_epoch
ORDER BY timestamp ASC;
EOF
}

format_events_for_prompt() {
    local events_file=$1
    local formatted_file=$2

    python3 <<EOF > "$formatted_file"
import json
import sys

events = []
with open('$events_file', 'r') as f:
    for line in f:
        line = line.strip()
        if line:
            events.append(json.loads(line))

grouped = {}
for evt in events:
    source = evt['source']
    if source not in grouped:
        grouped[source] = []
    grouped[source].append(evt)

source_priority = {
    'claude': 0,
    'github': 1,
    'git': 2,
    'kubectl': 2,
    'manual': 2,
    'shell': 3,
    'clipboard': 3,
    'tmux': 3,
    'wisprflow': 3
}

source_labels = {
    'claude': 'CRITICAL',
    'github': 'HIGH',
    'git': 'MEDIUM',
    'kubectl': 'MEDIUM',
    'manual': 'MEDIUM',
    'shell': 'LOW',
    'clipboard': 'LOW',
    'tmux': 'LOW',
    'wisprflow': 'LOW'
}

for source in sorted(grouped.keys(), key=lambda s: source_priority.get(s, 999)):
    evts = grouped[source]
    label = source_labels.get(source, 'MEDIUM')
    print(f"\n=== {label}: {source} ({len(evts)} events) ===")

    for evt in evts:
        payload = json.loads(evt.get('payload', '{}'))

        line = f"\n[{evt['timestamp']}] {evt['source']}/{evt['type']}"

        if evt.get('repo'):
            line += f" (repo: {evt['repo']})"
        if evt.get('branch'):
            line += f" (branch: {evt['branch']})"

        if 'workdir' in payload and payload['workdir']:
            line += f" (workdir: {payload['workdir']})"

        if 'summary' in payload and payload['summary']:
            summary = payload['summary']
            if len(summary) > 200:
                summary = summary[:200] + "..."
            line += f": {summary}"
        elif 'message' in payload and payload['message']:
            line += f": {payload['message']}"
        elif 'command' in payload and payload['command']:
            line += f": {payload['command']}"
        elif 'text' in payload and payload['text']:
            text = payload['text']
            if len(text) > 100:
                text = text[:100] + "..."
            line += f": {text}"

        print(line)
EOF
}

build_prompt() {
    local context_events=$1
    local focus_events=$2

    cat <<'EOF'
You are generating a factual development summary. This is a deterministic
transformation of the provided events, not a creative task. You must ONLY use
information explicitly present in the events. Never guess, infer intent, or
invent missing details.

You will be given two sets of events:

1. CONTEXT EVENTS — older events for background reference only
2. FOCUS EVENTS — the period that MUST be summarized

Events are grouped by source category:
- CRITICAL: Claude Code conversations, major architectural work
- HIGH: GitHub commits, PR activity
- MEDIUM: git commands, kubectl operations
- LOW: shell commands, clipboard activity, misc background

CONTEXT EVENTS (read for background only; DO NOT summarize these):
EOF
    echo "$context_events"
    cat <<'EOF'

FOCUS EVENTS (summarize ONLY these):
EOF
    echo "$focus_events"
    cat <<'EOF'

==================== SUMMARY REQUIREMENTS ====================

Your output has exactly two parts:

----------------------------------------------------------------
PART 1 — CONTEXT LINE (one line, max 80 chars)

Format:
"Working on: <repo> (<branch>)"

Rules:
- Extract repo and branch ONLY from focus events
- If multiple repos: use the one with most CRITICAL/HIGH activity
- If no repo/branch: use "Working on: <primary-topic>"
- Never use asterisks or markdown formatting in the context line
----------------------------------------------------------------

PART 2 — ACTIVITY SUMMARY (2–4 bullet points)

Each bullet MUST:
- Be one complete sentence in past tense
- Start with a strong action verb (not "Implemented clipboard operations" but specific action)
- Include technical specifics: file paths, function names, tool names, error messages
- Consolidate repetitive actions into patterns
- Focus on what was accomplished, not what was attempted

==================== SPECIFICITY GUIDELINES ====================

LEVEL OF DETAIL (aim for the middle):

TOO VAGUE ❌:
- "Implemented clipboard copy operations throughout the session"
- "Discussed and planned model testing for summarizer plugin"
- "Ran multiple terraform plans to manage AWS infrastructure"

TOO DETAILED ❌:
- "Executed terraform plan at 11:41:03, 11:41:19, 11:41:45, and 11:41:58"
- "Ran ./scripts/benchmark_summarizer.sh at 2025-11-20 04:28:39 and 13:47:21"
- "Copied various output logs and configurations related to script testing"

JUST RIGHT ✅:
- "Created benchmark script for testing LLM models on summarizer prompt variants"
- "Debugged terraform lock issue using force-unlock, then validated infrastructure plan"
- "Evaluated qwen2.5:14b and llama3.1:8b for summarization quality and speed"

==================== CONSOLIDATION RULES ====================

You MUST consolidate repetitive or similar events:
- If >3 related operations → describe the goal, not each operation
- Repetitive debugging → "Debugged <specific-issue>" with outcome if known
- Multiple commands for same goal → one bullet describing the objective
- Clipboard/shell spam → OMIT unless it reveals important pattern

EXAMPLES:
- NOT: "Ran benchmark script twice"
- YES: "Benchmarked multiple LLM models for summarizer performance"

- NOT: "Addressed Terraform lock issues by unlocking specific resource"
- YES: "Resolved terraform state lock conflict in aws-accounts-infra"

==================== PRIORITIZATION (STRICT ORDER) ====================

1. CRITICAL: architectural decisions, major code discussions
2. HIGH: commits, PRs, major git operations
3. MEDIUM: include ONLY if needed for understanding CRITICAL/HIGH
4. LOW: include only if pattern reveals clear intent

If a lower-priority event does not add value to understanding what was accomplished, OMIT IT.

==================== HARD RULES (DO NOT BREAK THESE) ====================

NEVER use:
- "the user", "I", "we", "they"
- Uncertainty: "appears", "seems", "probably", "likely"
- Meta phrases: "worked on", "focused on", "spent time", "continued to"
- Vague actions: "made changes", "updated files", "ran commands"
- Timestamps in bullets (dates are already in event format)
- Generic accomplishments without specifics

ALWAYS use:
- Past tense action verbs
- Specific file paths when relevant
- Tool/command names when they identify the work
- Technical terminology appropriate to the domain
- Concrete outcomes when visible in events

==================== OUTPUT FORMAT (STRICT) ====================

<one-line context>

- <bullet 1: most significant technical work>
- <bullet 2: second most significant work>
- <bullet 3: additional work if meaningfully different>
- <bullet 4: only if truly distinct from above>

Generate the summary now. Follow ALL rules above with zero deviations.
EOF
}

call_ollama() {
    local model=$1
    local prompt=$2

    local request_body
    request_body=$(jq -n \
        --arg model "$model" \
        --arg prompt "$prompt" \
        '{
            model: $model,
            messages: [{role: "user", content: $prompt}],
            stream: false,
            options: {
                temperature: 0.7,
                num_predict: 500
            }
        }')

    local response
    response=$(curl -s -X POST "${OLLAMA_BASE_URL}/api/chat" \
        -H "Content-Type: application/json" \
        -d "$request_body")

    local summary
    summary=$(echo "$response" | jq -r '.message.content // empty')

    local error
    error=$(echo "$response" | jq -r '.error // empty')

    if [ -n "$error" ]; then
        echo "ERROR: $error"
        return 1
    fi

    if [ -z "$summary" ]; then
        echo "ERROR: No response from model"
        return 1
    fi

    echo "$summary"
}

date_to_epoch() {
    local date_str=$1
    if [[ "$OSTYPE" == "darwin"* ]]; then
        date -j -f "%Y-%m-%d %H:%M" "$date_str" +%s
    else
        date -d "$date_str" +%s
    fi
}

epoch_to_date() {
    local epoch=$1
    if [[ "$OSTYPE" == "darwin"* ]]; then
        date -r "$epoch" "+%Y-%m-%d %H:%M:%S"
    else
        date -d "@$epoch" "+%Y-%m-%d %H:%M:%S"
    fi
}

format_time() {
    local epoch=$1
    if [[ "$OSTYPE" == "darwin"* ]]; then
        date -r "$epoch" "+%H:%M"
    else
        date -d "@$epoch" "+%H:%M"
    fi
}

generate_summaries() {
    local start_epoch end_epoch

    if [ -n "$START_TIME" ] && [ -n "$END_TIME" ]; then
        start_epoch=$(date_to_epoch "$START_TIME")
        end_epoch=$(date_to_epoch "$END_TIME")
    else
        end_epoch=$(date +%s)
        start_epoch=$((end_epoch - (HOURS_BACK * 3600)))
    fi

    local output_file="$OUTPUT_FILE"
    if [ -z "$output_file" ]; then
        local timestamp=$(date +%Y%m%d_%H%M%S)
        output_file="${PROJECT_ROOT}/benchmark_results/historical_summary_${timestamp}.md"
    fi

    mkdir -p "$(dirname "$output_file")"

    log_info "Generating historical summaries..."
    log_info "  Time range: $(epoch_to_date $start_epoch) to $(epoch_to_date $end_epoch)"
    log_info "  Interval: ${INTERVAL_MINUTES} minutes"
    log_info "  Context window: ${CONTEXT_MINUTES} minutes"
    log_info "  Model: ${MODEL}"
    log_info "  Output: $output_file"
    echo

    cat > "$output_file" <<EOF
# Historical Development Summary

**Generated:** $(date)
**Time Range:** $(epoch_to_date $start_epoch) to $(epoch_to_date $end_epoch)
**Interval:** ${INTERVAL_MINUTES} minutes
**Context Window:** ${CONTEXT_MINUTES} minutes
**Model:** ${MODEL}
**Database:** $DB_PATH

---

EOF

    local current_epoch=$start_epoch
    local interval_seconds=$((INTERVAL_MINUTES * 60))
    local context_seconds=$((CONTEXT_MINUTES * 60))
    local total_intervals=$(( (end_epoch - start_epoch) / interval_seconds ))
    local current_interval=0

    local temp_dir=$(mktemp -d)
    trap "rm -rf $temp_dir" EXIT

    while [ $current_epoch -lt $end_epoch ]; do
        current_interval=$((current_interval + 1))
        local focus_start=$current_epoch
        local focus_end=$((current_epoch + interval_seconds))
        local context_start=$((current_epoch - context_seconds))

        if [ $context_start -lt $start_epoch ]; then
            context_start=$start_epoch
        fi

        log_info "[${current_interval}/${total_intervals}] Processing $(format_time $focus_start) - $(format_time $focus_end)"

        local context_events_file="${temp_dir}/context_events.json"
        local focus_events_file="${temp_dir}/focus_events.json"
        local context_formatted="${temp_dir}/context_formatted.txt"
        local focus_formatted="${temp_dir}/focus_formatted.txt"

        fetch_events_in_range $context_start $focus_start "$context_events_file"
        fetch_events_in_range $focus_start $focus_end "$focus_events_file"

        local focus_count
        focus_count=$(wc -l < "$focus_events_file" | tr -d ' ')

        if [ "$focus_count" -eq 0 ]; then
            log_info "  No events in focus window, skipping..."
            cat >> "$output_file" <<EOF
## $(format_time $focus_start) - $(format_time $focus_end)

_No development activity recorded during this period._

---

EOF
            current_epoch=$focus_end
            continue
        fi

        format_events_for_prompt "$context_events_file" "$context_formatted"
        format_events_for_prompt "$focus_events_file" "$focus_formatted"

        local context_content
        local focus_content
        context_content=$(cat "$context_formatted")
        focus_content=$(cat "$focus_formatted")

        local prompt
        prompt=$(build_prompt "$context_content" "$focus_content")

        log_info "  Found ${focus_count} focus events, calling LLM..."

        local summary
        if summary=$(call_ollama "$MODEL" "$prompt"); then
            log_success "  Generated summary"

            cat >> "$output_file" <<EOF
## $(format_time $focus_start) - $(format_time $focus_end)

$summary

---

EOF
        else
            log_error "  Failed to generate summary: $summary"

            cat >> "$output_file" <<EOF
## $(format_time $focus_start) - $(format_time $focus_end)

_Summary generation failed: $summary_

---

EOF
        fi

        current_epoch=$focus_end
    done

    log_success "Historical summaries generated successfully!"
    log_info "Output saved to: $output_file"
}

show_help() {
    cat <<EOF
Usage: $0 [OPTIONS]

Generate summaries for historical time periods to show what summary logs
would have looked like if the summarizer was running at that time.

Options:
    -h, --help              Show this help message
    -m, --model MODEL       Model to use (default: qwen2.5:14b)
    --hours HOURS           Hours to look back (default: 24)
    --interval MINS         Interval in minutes (default: 15)
    --context MINS          Context window in minutes (default: 60)
    --start TIMESTAMP       Start time (format: "YYYY-MM-DD HH:MM")
    --end TIMESTAMP         End time (format: "YYYY-MM-DD HH:MM")
    -d, --db-path PATH      Path to events database
    -o, --output FILE       Output file path
    --ollama-url URL        Ollama base URL

Examples:
    # Generate summaries for last 24 hours (default)
    $0

    # Generate for last 48 hours with 30-minute intervals
    $0 --hours 48 --interval 30

    # Generate for specific date range
    $0 --start "2025-11-19 09:00" --end "2025-11-19 17:00"

    # Use different model
    $0 --model qwen2.5:7b

Environment Variables:
    OLLAMA_BASE_URL         Override default Ollama URL

EOF
}

main() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -m|--model)
                MODEL="$2"
                shift 2
                ;;
            --hours)
                HOURS_BACK="$2"
                shift 2
                ;;
            --interval)
                INTERVAL_MINUTES="$2"
                shift 2
                ;;
            --context)
                CONTEXT_MINUTES="$2"
                shift 2
                ;;
            --start)
                START_TIME="$2"
                shift 2
                ;;
            --end)
                END_TIME="$2"
                shift 2
                ;;
            -d|--db-path)
                DB_PATH="$2"
                shift 2
                ;;
            -o|--output)
                OUTPUT_FILE="$2"
                shift 2
                ;;
            --ollama-url)
                OLLAMA_BASE_URL="$2"
                shift 2
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done

    log_info "Devlog Historical Summarizer"
    log_info "============================"
    echo

    check_ollama
    check_database

    if ! pull_model "$MODEL"; then
        log_error "Failed to pull model ${MODEL}"
        exit 1
    fi

    echo
    read -p "Generate historical summaries? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Cancelled"
        exit 0
    fi

    generate_summaries
}

if ! command -v sqlite3 &> /dev/null; then
    log_error "sqlite3 is required but not installed"
    exit 1
fi

if ! command -v jq &> /dev/null; then
    log_error "jq is required but not installed"
    exit 1
fi

if ! command -v python3 &> /dev/null; then
    log_error "python3 is required but not installed"
    exit 1
fi

if ! command -v bc &> /dev/null; then
    log_error "bc is required but not installed"
    exit 1
fi

main "$@"
