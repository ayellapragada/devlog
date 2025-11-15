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

__devlog_get_repo_info() {
    REPO_PATH="$(/usr/bin/git rev-parse --show-toplevel 2>/dev/null)"
    BRANCH="$(/usr/bin/git rev-parse --abbrev-ref HEAD 2>/dev/null)"
    [ "$BRANCH" = "HEAD" ] && BRANCH="detached-$(/usr/bin/git rev-parse --short HEAD 2>/dev/null)"
}

__devlog_capture_git_event() {
    local event_type="$1"
    local repo_path="$2"
    local branch="$3"
    shift 3
    local extra_args="$@"

    local devlog_bin="$(__devlog_find_bin)"
    [ -z "$devlog_bin" ] && return

    (
        "$devlog_bin" ingest git-event \
            --type="$event_type" \
            --repo="$repo_path" \
            --branch="$branch" \
            $extra_args \
            &> /dev/null
    ) &
}
