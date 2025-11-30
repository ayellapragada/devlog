#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMMON_LIB="${SCRIPT_DIR}/devlog-tmux-common.sh"

if [ -f "$COMMON_LIB" ]; then
    source "$COMMON_LIB"
elif [ -f "${HOME}/.config/devlog/devlog-tmux-common.sh" ]; then
    source "${HOME}/.config/devlog/devlog-tmux-common.sh"
else
    exit 0
fi

__devlog_capture_tmux_event "$@"

