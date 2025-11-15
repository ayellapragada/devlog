#!/bin/bash

DEVLOG_SHELL_ENABLED="${DEVLOG_SHELL_ENABLED:-true}"
[ "$DEVLOG_SHELL_ENABLED" != "true" ] && return

__devlog_find_bin() {
    local devlog_bin="${DEVLOG_BIN:-devlog}"

    if command -v "$devlog_bin" &> /dev/null; then
        echo "$devlog_bin"
        return 0
    fi

    for path in /usr/local/bin/devlog ~/.local/bin/devlog ~/bin/devlog ./bin/devlog; do
        if [ -x "$path" ]; then
            echo "$path"
            return 0
        fi
    done

    return 1
}

DEVLOG_BIN_PATH=$(__devlog_find_bin)

if [ -n "$DEVLOG_BIN_PATH" ]; then
    __devlog_preexec() {
        export DEVLOG_CMD_START=$(date +%s)
        export DEVLOG_CMD="$1"
    }

    __devlog_precmd() {
        local exit_code=$?

        [ -z "$DEVLOG_CMD" ] && return

        [ -z "$(echo "$DEVLOG_CMD" | tr -d '[:space:]')" ] && {
            unset DEVLOG_CMD DEVLOG_CMD_START
            return
        }

        # Skip devlog daemon control commands to avoid consistent "always processing" messages
        if echo "$DEVLOG_CMD" | grep -qE '(^|[[:space:]])devlog[[:space:]]+daemon[[:space:]]+(start|stop|restart|status)'; then
            unset DEVLOG_CMD DEVLOG_CMD_START
            return
        fi

        local duration=0
        if [ -n "$DEVLOG_CMD_START" ]; then
            local end_time=$(date +%s)
            local seconds=$((end_time - DEVLOG_CMD_START))
            duration=$((seconds * 1000))
        fi

        local workdir="$PWD"

        if [ -n "$ZSH_VERSION" ]; then
            {
                "$DEVLOG_BIN_PATH" ingest shell-command \
                    --command="$DEVLOG_CMD" \
                    --exit-code="$exit_code" \
                    --workdir="$workdir" \
                    --duration="$duration" \
                    &> /dev/null &
            } &!
        else
            (
                "$DEVLOG_BIN_PATH" ingest shell-command \
                    --command="$DEVLOG_CMD" \
                    --exit-code="$exit_code" \
                    --workdir="$workdir" \
                    --duration="$duration" \
                    &> /dev/null
            ) &
        fi

        unset DEVLOG_CMD DEVLOG_CMD_START
    }

    if [ -n "$BASH_VERSION" ]; then
        __devlog_bash_preexec() {
            [ -n "$COMP_LINE" ] && return  # Skip if in completion
            [ "$BASH_COMMAND" = "$PROMPT_COMMAND" ] && return  # Skip PROMPT_COMMAND itself

            local cmd="$BASH_COMMAND"
            __devlog_preexec "$cmd"
        }

        trap '__devlog_bash_preexec' DEBUG

        if [[ "$PROMPT_COMMAND" != *"__devlog_precmd"* ]]; then
            PROMPT_COMMAND="__devlog_precmd${PROMPT_COMMAND:+; $PROMPT_COMMAND}"
        fi
    fi

    if [ -n "$ZSH_VERSION" ]; then
        autoload -Uz add-zsh-hook

        add-zsh-hook preexec __devlog_preexec
        add-zsh-hook precmd __devlog_precmd
    fi
fi
