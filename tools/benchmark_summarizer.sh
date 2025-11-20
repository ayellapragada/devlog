#!/bin/bash
# =============================================================================
# Devlog Summarizer Benchmark Tool
# =============================================================================
#
# Tests different LLM models and prompt variants to find optimal summarizer
# configuration. Queries events from your database and measures quality, speed,
# and format adherence.
#
# USAGE:
#   ./tools/benchmark_summarizer.sh [OPTIONS]
#
# EXAMPLES:
#   # Quick test with small models
#   ./tools/benchmark_summarizer.sh --models qwen2.5:7b --counts 25
#
#   # Test multiple models at scale
#   ./tools/benchmark_summarizer.sh \
#     --models qwen2.5:7b qwen2.5:14b qwen2.5:32b \
#     --counts 10 25 50 100
#
#   # Keep models in memory (faster, more RAM)
#   ./tools/benchmark_summarizer.sh --keep-loaded
#
# OPTIONS:
#   -h, --help              Show this help
#   -m, --models MODEL...   Models to test (default: 8 models)
#   -c, --counts COUNT...   Event counts (default: 10,25,50,100)
#   -d, --db-path PATH      Database path
#   -o, --output DIR        Results directory
#   --keep-loaded           Don't unload models (uses more RAM)
#   --ollama-url URL        Ollama server URL
#
# PROMPT VARIANTS:
#   Each model is tested with 5 prompts:
#   - original: Current production prompt
#   - concise: Minimal instructions
#   - detailed: Technical focus
#   - minimal: Baseline test
#   - structured: Explicit sections
#
# MEMORY MANAGEMENT:
#   Models auto-unload after testing (default):
#   - qwen2.5:7b  → 4-6GB
#   - qwen2.5:14b → 8-10GB (recommended)
#   - qwen2.5:32b → 18-24GB
#
# OUTPUT:
#   Results: benchmark_results/benchmark_TIMESTAMP.md
#
# REQUIREMENTS:
#   - ollama (running: ollama serve)
#   - sqlite3, jq, python3, bc
#   - Database with events at ~/.local/share/devlog/events.db
#
# MORE INFO:
#   Run with --help to see this message
#   Check benchmark_results/ for past runs
#   Use analyze_benchmark.py to compare results
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
RESULTS_DIR="${PROJECT_ROOT}/benchmark_results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULTS_FILE="${RESULTS_DIR}/benchmark_${TIMESTAMP}.md"

MODELS=(
    "qwen2.5:3b"
    "qwen2.5:7b"
    "qwen2.5:14b"
    "qwen2.5:32b"
    "llama3.2:3b"
    "llama3.1:8b"
    "gemma2:9b"
    "mistral:7b"
)

EVENT_COUNTS=(10 25 50 100)

UNLOAD_MODELS=true

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

    if [ "$count" -lt 10 ]; then
        log_warn "Database has fewer than 10 events. Benchmarking may not be meaningful."
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

unload_model() {
    local model=$1
    log_info "Unloading model ${model} from memory to free resources..."

    local unload_response
    unload_response=$(curl -s -X POST "${OLLAMA_BASE_URL}/api/generate" \
        -H "Content-Type: application/json" \
        -d "{\"model\": \"${model}\", \"keep_alive\": 0}")

    if [ $? -eq 0 ]; then
        log_success "Model ${model} unloaded from memory"
        return 0
    else
        log_warn "Failed to unload ${model} (non-fatal, will be unloaded eventually)"
        return 1
    fi
}

fetch_events() {
    local count=$1
    local output_file=$2

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
WHERE timestamp >= strftime('%s', 'now', '-7 days')
ORDER BY timestamp DESC
LIMIT $count;
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

events = events[::-1]

for evt in events:
    payload = json.loads(evt.get('payload', '{}'))

    line = f"\n[{evt['timestamp']}] {evt['source']}/{evt['type']}"

    if evt.get('repo'):
        line += f" (repo: {evt['repo']})"
    if evt.get('branch'):
        line += f" (branch: {evt['branch']})"

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

build_prompt_variant() {
    local variant=$1
    local events_content=$2

    case "$variant" in
        "original")
            cat <<EOF
You are summarizing a development session. Here are the FOCUS EVENTS you need to summarize:

$events_content

===== INSTRUCTIONS =====

Generate a two-part summary:

PART 1 - Context line (max 80 characters):
State the project/repository and branch being worked on. Use format: "Working on: <repo> (<branch>)"

PART 2 - Activity summary (Between 2 and 4 bullet points):
Each bullet must be a complete sentence in past tense describing what was accomplished.

CONSOLIDATION RULES:
- If >3 similar commands/operations: "Ran multiple <type> checks" or "Made several <component> changes"
- If repetitive debugging: "Debugged <issue>" not a list of each debug command
- If exploring/researching: "Investigated <topic> in <files>" not each individual grep/read

PRIORITIZATION (in order of importance):
1. CRITICAL events: Major design decisions, architectural discussions, feature implementations
2. HIGH events: Code commits, PR creation, significant git operations
3. MEDIUM events: Only include if they provide essential context to CRITICAL/HIGH events
4. LOW events: Omit unless they reveal a clear pattern (e.g., "Monitored Kubernetes cluster health")

WRITING RULES:
✓ Use past tense: "Implemented", "Fixed", "Refactored", "Discussed"
✓ Be specific: Include file names, component names, function names when relevant
✓ Be direct: Start bullets with action verbs
✓ Be dense: Pack multiple related actions into one bullet

✗ Never use: "the user", "they", "I", "we", "appears", "seems", "likely", "probably"
✗ No meta-commentary: "Focused on", "Worked on", "Continued to", "Spent time"
✗ No vague actions: "Made changes", "Updated files", "Ran commands"
✗ No question marks or uncertainty

===== OUTPUT FORMAT =====

<One line context, max 80 chars>

- <Bullet point 1: most important activity>
- <Bullet point 2: second most important activity>
- <Bullet point 3: third most important activity IF NEEDED>
- <Bullet point 4: fourth most important activity IF NEEDED>

Generate the summary now, following ALL rules above:
EOF
            ;;
        "concise")
            cat <<EOF
Summarize this development session in 3 bullets (past tense):

$events_content

Format:
<One line: project/repo on branch>

- <Most important activity>
- <Second most important>
- <Third most important>
EOF
            ;;
        "detailed")
            cat <<EOF
You are analyzing development activity. Review these events and create a detailed summary.

EVENTS:
$events_content

INSTRUCTIONS:
1. Identify the primary repository and branch
2. List the 3 most significant activities in order of importance
3. Include specific file names, functions, or components when relevant
4. Use past tense and active voice
5. Be technical and precise

OUTPUT FORMAT:
Repository: <name> on <branch>

1. <Most significant technical activity with specifics>
2. <Second most significant activity>
3. <Third most significant activity>
EOF
            ;;
        "minimal")
            cat <<EOF
Events:
$events_content

Summarize in 3 bullets what was accomplished.
EOF
            ;;
        "structured")
            cat <<EOF
Development Session Summary

Events to analyze:
$events_content

Required output structure:

## Context
- Repository: [name]
- Branch: [name]
- Time period: [inferred from events]

## Key Activities (3 items in past tense)
1. [Primary focus area with technical details]
2. [Secondary activity]
3. [Additional work completed]

Rules:
- Be specific and technical
- Include file/component names
- Use active voice, past tense
- No speculation or uncertainty
EOF
            ;;
    esac
}

call_ollama() {
    local model=$1
    local prompt=$2
    local output_file=$3

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

    local start_time
    if [[ "$OSTYPE" == "darwin"* ]]; then
        start_time=$(python3 -c 'import time; print(int(time.time() * 1000))')
    else
        start_time=$(date +%s%3N)
    fi

    local response
    response=$(curl -s -X POST "${OLLAMA_BASE_URL}/api/chat" \
        -H "Content-Type: application/json" \
        -d "$request_body")

    local end_time
    if [[ "$OSTYPE" == "darwin"* ]]; then
        end_time=$(python3 -c 'import time; print(int(time.time() * 1000))')
    else
        end_time=$(date +%s%3N)
    fi
    local duration=$((end_time - start_time))

    local summary
    summary=$(echo "$response" | jq -r '.message.content // empty')

    local error
    error=$(echo "$response" | jq -r '.error // empty')

    if [ -n "$error" ]; then
        echo "ERROR: $error" > "$output_file"
        echo "$duration"
        return 1
    fi

    if [ -z "$summary" ]; then
        echo "ERROR: No response from model" > "$output_file"
        echo "$duration"
        return 1
    fi

    echo "$summary" > "$output_file"
    echo "$duration"
}

run_benchmark() {
    mkdir -p "$RESULTS_DIR"

    log_info "Starting benchmark at $(date)"
    log_info "Results will be saved to: $RESULTS_FILE"

    cat > "$RESULTS_FILE" <<EOF
# Summarizer Benchmark Results

**Date:** $(date)
**Database:** $DB_PATH
**Ollama URL:** $OLLAMA_BASE_URL

## Configuration

### Models Tested
$(for model in "${MODELS[@]}"; do echo "- $model"; done)

### Event Counts
$(for count in "${EVENT_COUNTS[@]}"; do echo "- $count events"; done)

### Prompt Variants
- original: Full detailed prompt with comprehensive rules
- concise: Minimal instructions, direct format
- detailed: Technical focus with structured output
- minimal: Bare minimum instructions
- structured: Markdown-formatted structured output

---

EOF

    local total_tests=$((${#MODELS[@]} * ${#EVENT_COUNTS[@]} * 5))
    local current_test=0

    for model in "${MODELS[@]}"; do
        log_info "Testing model: ${model}"

        if ! pull_model "$model"; then
            log_warn "Skipping model ${model}"
            echo "## ${model}" >> "$RESULTS_FILE"
            echo "" >> "$RESULTS_FILE"
            echo "**Status:** Model unavailable or failed to pull" >> "$RESULTS_FILE"
            echo "" >> "$RESULTS_FILE"
            continue
        fi

        echo "## ${model}" >> "$RESULTS_FILE"
        echo "" >> "$RESULTS_FILE"

        for count in "${EVENT_COUNTS[@]}"; do
            log_info "  Testing with ${count} events"

            echo "### ${count} Events" >> "$RESULTS_FILE"
            echo "" >> "$RESULTS_FILE"

            local events_json_file="${RESULTS_DIR}/events_${count}.json"
            local events_formatted_file="${RESULTS_DIR}/events_${count}_formatted.txt"

            fetch_events "$count" "$events_json_file"
            format_events_for_prompt "$events_json_file" "$events_formatted_file"

            local events_content
            events_content=$(cat "$events_formatted_file")

            for variant in "original" "concise" "detailed" "minimal" "structured"; do
                current_test=$((current_test + 1))
                log_info "    [${current_test}/${total_tests}] Variant: ${variant}"

                local prompt
                prompt=$(build_prompt_variant "$variant" "$events_content")

                local model_safe
                model_safe=$(echo "$model" | tr ':' '_')
                local output_file="${RESULTS_DIR}/output_${model_safe}_${count}_${variant}.txt"

                local duration
                if duration=$(call_ollama "$model" "$prompt" "$output_file"); then
                    local summary
                    summary=$(cat "$output_file")

                    local duration_sec
                    duration_sec=$(echo "scale=2; $duration / 1000" | bc)

                    log_success "      Completed in ${duration_sec}s"

                    cat >> "$RESULTS_FILE" <<EOF
#### Variant: ${variant}

**Duration:** ${duration_sec}s

**Summary:**
\`\`\`
$summary
\`\`\`

---

EOF
                else
                    local error_msg
                    error_msg=$(cat "$output_file")
                    log_error "      Failed: $error_msg"

                    cat >> "$RESULTS_FILE" <<EOF
#### Variant: ${variant}

**Duration:** N/A
**Status:** Failed
**Error:** $error_msg

---

EOF
                fi
            done

            echo "" >> "$RESULTS_FILE"
        done

        if [ "$UNLOAD_MODELS" = true ]; then
            unload_model "$model"
        fi

        echo "" >> "$RESULTS_FILE"
    done

    log_success "Benchmark complete! Results saved to: $RESULTS_FILE"

    cat >> "$RESULTS_FILE" <<EOF

---

## Analysis

To analyze these results, consider:

1. **Quality**: Which summaries are most accurate and useful?
2. **Consistency**: Which model/prompt combinations produce reliable output?
3. **Performance**: Balance between speed and quality
4. **Cost**: Model size vs. quality tradeoff
5. **Format Adherence**: Which variants follow instructions best?

### Recommended Next Steps

1. Review summaries for factual accuracy against actual events
2. Check if summaries follow the 3-bullet format consistently
3. Evaluate technical specificity and detail level
4. Compare performance across different event counts
5. Select best model/prompt combination for production use

EOF
}

show_help() {
    cat <<EOF
Usage: $0 [OPTIONS]

Benchmark different LLM models and prompt variations for the devlog summarizer plugin.

Options:
    -h, --help              Show this help message
    -m, --models MODEL...   Specify models to test (default: qwen2.5:3b,7b,14b,32b + others)
    -c, --counts COUNT...   Specify event counts to test (default: 10,25,50,100)
    -d, --db-path PATH      Path to events database (default: ~/.local/share/devlog/events.db)
    -o, --output DIR        Output directory for results (default: ./benchmark_results)
    --ollama-url URL        Ollama base URL (default: http://localhost:11434)
    --keep-loaded           Keep models in memory after testing (saves memory by default)

Examples:
    # Run with default settings (unloads models after each test)
    $0

    # Test only specific models
    $0 --models qwen2.5:7b llama3.1:8b

    # Test with custom event counts
    $0 --counts 10 50 100

    # Keep models loaded in memory (useful for back-to-back runs)
    $0 --keep-loaded

    # Use custom database path
    $0 --db-path /path/to/events.db

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
            -m|--models)
                MODELS=()
                shift
                while [[ $# -gt 0 ]] && [[ ! $1 =~ ^- ]]; do
                    MODELS+=("$1")
                    shift
                done
                ;;
            -c|--counts)
                EVENT_COUNTS=()
                shift
                while [[ $# -gt 0 ]] && [[ ! $1 =~ ^- ]]; do
                    EVENT_COUNTS+=("$1")
                    shift
                done
                ;;
            -d|--db-path)
                DB_PATH="$2"
                shift 2
                ;;
            -o|--output)
                RESULTS_DIR="$2"
                shift 2
                ;;
            --ollama-url)
                OLLAMA_BASE_URL="$2"
                shift 2
                ;;
            --keep-loaded)
                UNLOAD_MODELS=false
                shift
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done

    log_info "Devlog Summarizer Benchmark"
    log_info "============================"
    echo

    check_ollama
    check_database

    log_info "Configuration:"
    log_info "  Models: ${MODELS[*]}"
    log_info "  Event counts: ${EVENT_COUNTS[*]}"
    log_info "  Database: $DB_PATH"
    log_info "  Results: $RESULTS_DIR"
    echo

    read -p "Start benchmark? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Benchmark cancelled"
        exit 0
    fi

    run_benchmark
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
