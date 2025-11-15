#!/bin/bash
# devlog shell integration
# Automatically captures shell commands and sends them to devlogd
#
# Installation:
#   Add to your ~/.bashrc or ~/.zshrc:
#   source /path/to/devlog/hooks/devlog.sh
#
# Configuration:
#   export DEVLOG_BIN=/path/to/devlog      # Custom devlog binary path
#   export DEVLOG_SHELL_ENABLED=true       # Enable/disable (default: true)

# Check if devlog is enabled
DEVLOG_SHELL_ENABLED="${DEVLOG_SHELL_ENABLED:-true}"
[ "$DEVLOG_SHELL_ENABLED" != "true" ] && return

# Find devlog binary
__devlog_find_bin() {
    local devlog_bin="${DEVLOG_BIN:-devlog}"

    if command -v "$devlog_bin" &> /dev/null; then
        echo "$devlog_bin"
        return 0
    fi

    # Check common locations
    for path in /usr/local/bin/devlog ~/.local/bin/devlog ~/bin/devlog ./bin/devlog; do
        if [ -x "$path" ]; then
            echo "$path"
            return 0
        fi
    done

    return 1
}

# Store the devlog binary path once
DEVLOG_BIN_PATH=$(__devlog_find_bin)

# Only set up hooks if devlog is available
if [ -n "$DEVLOG_BIN_PATH" ]; then
    # Preexec: Store command before execution
    __devlog_preexec() {
        # Capture the command being executed
        export DEVLOG_CMD_START=$(date +%s)
        export DEVLOG_CMD="$1"
    }

    # Precmd: Capture result after execution
    __devlog_precmd() {
        local exit_code=$?

        # Skip if no command was captured
        [ -z "$DEVLOG_CMD" ] && return

        # Skip empty commands
        [ -z "$(echo "$DEVLOG_CMD" | tr -d '[:space:]')" ] && {
            unset DEVLOG_CMD DEVLOG_CMD_START
            return
        }

        # Calculate duration in milliseconds
        local duration=0
        if [ -n "$DEVLOG_CMD_START" ]; then
            local end_time=$(date +%s)
            local seconds=$((end_time - DEVLOG_CMD_START))
            duration=$((seconds * 1000))
        fi

        # Get working directory
        local workdir="$PWD"

        # Send to devlog in background (don't block shell)
        if [ -n "$ZSH_VERSION" ]; then
            # Zsh: use &! to background and immediately disown
            {
                "$DEVLOG_BIN_PATH" ingest shell-command \
                    --command="$DEVLOG_CMD" \
                    --exit-code="$exit_code" \
                    --workdir="$workdir" \
                    --duration="$duration" \
                    &> /dev/null &
            } &!
        else
            # Bash: use subshell to prevent job control messages
            (
                "$DEVLOG_BIN_PATH" ingest shell-command \
                    --command="$DEVLOG_CMD" \
                    --exit-code="$exit_code" \
                    --workdir="$workdir" \
                    --duration="$duration" \
                    &> /dev/null
            ) &
        fi

        # Clear stored command
        unset DEVLOG_CMD DEVLOG_CMD_START
    }

    # Setup for Bash
    if [ -n "$BASH_VERSION" ]; then
        # Bash doesn't have native preexec, so we use DEBUG trap
        __devlog_bash_preexec() {
            # Only capture if we're at an interactive prompt
            [ -n "$COMP_LINE" ] && return  # Skip if in completion
            [ "$BASH_COMMAND" = "$PROMPT_COMMAND" ] && return  # Skip PROMPT_COMMAND itself

            # Get the command that's about to execute
            local cmd="$BASH_COMMAND"
            __devlog_preexec "$cmd"
        }

        # Set up DEBUG trap for preexec
        trap '__devlog_bash_preexec' DEBUG

        # Set up PROMPT_COMMAND for precmd
        if [[ "$PROMPT_COMMAND" != *"__devlog_precmd"* ]]; then
            PROMPT_COMMAND="__devlog_precmd${PROMPT_COMMAND:+; $PROMPT_COMMAND}"
        fi
    fi

    # Setup for Zsh
    if [ -n "$ZSH_VERSION" ]; then
        # Load zsh's hook functions
        autoload -Uz add-zsh-hook

        # Zsh has native preexec and precmd hooks
        add-zsh-hook preexec __devlog_preexec
        add-zsh-hook precmd __devlog_precmd
    fi
fi
