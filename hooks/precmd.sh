#!/bin/bash
# devlog precmd hook
# Captures shell command results after execution
# This script is meant to be sourced in your shell's rc file

__devlog_capture_command() {
    local exit_code=$?

    # Skip if no command was captured
    [ -z "$DEVLOG_CMD" ] && return

    # Find devlog binary
    local devlog_bin="${DEVLOG_BIN:-devlog}"
    if ! command -v "$devlog_bin" &> /dev/null; then
        # Check common locations
        for path in /usr/local/bin/devlog ~/.local/bin/devlog; do
            if [ -x "$path" ]; then
                devlog_bin="$path"
                break
            fi
        done
    fi

    # Exit if devlog not found
    command -v "$devlog_bin" &> /dev/null || return

    # Calculate duration
    local duration=0
    if [ -n "$DEVLOG_CMD_START" ]; then
        local end_time=$(date +%s%3N)
        duration=$((end_time - DEVLOG_CMD_START))
    fi

    # Get working directory
    local workdir="$PWD"

    # Send to devlog in background (don't block shell)
    "$devlog_bin" ingest shell-command \
        --command="$DEVLOG_CMD" \
        --exit-code="$exit_code" \
        --workdir="$workdir" \
        --duration="$duration" \
        &> /dev/null &

    # Clear stored command
    unset DEVLOG_CMD
    unset DEVLOG_CMD_START
}

# For bash
if [ -n "$BASH_VERSION" ]; then
    PROMPT_COMMAND="__devlog_capture_command${PROMPT_COMMAND:+; $PROMPT_COMMAND}"
fi

# For zsh
if [ -n "$ZSH_VERSION" ]; then
    precmd_functions+=(__devlog_capture_command)
fi
