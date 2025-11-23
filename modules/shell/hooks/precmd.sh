#!/bin/bash

__devlog_capture_command() {
    local exit_code=$?

    [ -z "$DEVLOG_CMD" ] && return

    local devlog_bin="${DEVLOG_BIN:-devlog}"
    if ! command -v "$devlog_bin" &> /dev/null; then
        for path in /usr/local/bin/devlog ~/.local/bin/devlog; do
            if [ -x "$path" ]; then
                devlog_bin="$path"
                break
            fi
        done
    fi

    command -v "$devlog_bin" &> /dev/null || return

    local duration=0
    if [ -n "$DEVLOG_CMD_START" ]; then
        local end_time=$(date +%s%3N)
        duration=$((end_time - DEVLOG_CMD_START))
    fi

    local workdir="$PWD"
    local branch=""
    if git rev-parse --git-dir &> /dev/null; then
        branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || git rev-parse --short HEAD 2>/dev/null)
    fi

    local args=(
        --command="$DEVLOG_CMD"
        --exit-code="$exit_code"
        --workdir="$workdir"
        --duration="$duration"
    )

    if [ -n "$branch" ]; then
        args+=(--branch="$branch")
    fi

    "$devlog_bin" ingest shell-command "${args[@]}" &> /dev/null &

    unset DEVLOG_CMD
    unset DEVLOG_CMD_START
}

if [ -n "$BASH_VERSION" ]; then
    PROMPT_COMMAND="__devlog_capture_command${PROMPT_COMMAND:+; $PROMPT_COMMAND}"
fi

if [ -n "$ZSH_VERSION" ]; then
    precmd_functions+=(__devlog_capture_command)
fi
