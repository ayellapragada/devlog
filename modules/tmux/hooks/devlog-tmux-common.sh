#!/bin/bash

__devlog_find_bin() {
    local devlog_bin="${DEVLOG_BIN:-devlog}"

    if command -v "$devlog_bin" &> /dev/null; then
        echo "$devlog_bin"
        return 0
    fi

    local home_dir="${HOME}"
    for path in /usr/local/bin/devlog "${home_dir}/.local/bin/devlog" "${home_dir}/bin/devlog" ./bin/devlog; do
        if [ -x "$path" ]; then
            echo "$path"
            return 0
        fi
    done

    return 1
}

__devlog_capture_tmux_event() {
    local devlog_bin="$(__devlog_find_bin)"
    [ -z "$devlog_bin" ] && return

    (
        "$devlog_bin" ingest tmux-event "$@" &> /dev/null
    ) &
}

