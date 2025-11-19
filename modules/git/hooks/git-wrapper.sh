#!/bin/bash

DEVLOG_GIT_ENABLED="${DEVLOG_GIT_ENABLED:-true}"

find_real_git() {
    local this_script="$(realpath "${BASH_SOURCE[0]}" 2>/dev/null || readlink -f "${BASH_SOURCE[0]}" 2>/dev/null)"
    [ -z "$this_script" ] && this_script="${BASH_SOURCE[0]}"

    IFS=: read -ra paths <<< "$PATH"
    for dir in "${paths[@]}"; do
        [ -z "$dir" ] && continue
        local candidate="$dir/git"
        [ ! -x "$candidate" ] && continue
        local candidate_real="$(realpath "$candidate" 2>/dev/null || readlink -f "$candidate" 2>/dev/null)"
        [ -z "$candidate_real" ] && candidate_real="$candidate"
        [ "$candidate_real" = "$this_script" ] && continue
        echo "$candidate"
        return 0
    done
    echo "/usr/bin/git"
}

GIT_BIN="$(find_real_git)"
[ "$DEVLOG_GIT_ENABLED" != "true" ] && exec "$GIT_BIN" "$@"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMMON_LIB="${SCRIPT_DIR}/devlog-git-common.sh"

if [ -f "$COMMON_LIB" ]; then
    source "$COMMON_LIB"
elif [ -f "${HOME}/.local/bin/devlog-git-common.sh" ]; then
    source "${HOME}/.local/bin/devlog-git-common.sh"
else
    exec "$GIT_BIN" "$@"
fi

SUBCOMMAND="$1"

case "$SUBCOMMAND" in
    commit)
        __devlog_get_repo_info
        "$GIT_BIN" "$@"
        EXIT_CODE=$?

        if [ $EXIT_CODE -eq 0 ] && [ -n "$REPO_PATH" ]; then
            read -r COMMIT_HASH COMMIT_AUTHOR < <("$GIT_BIN" log -1 --format='%H %an' 2>/dev/null)
            if [ -n "$COMMIT_HASH" ]; then
                COMMIT_MESSAGE="$("$GIT_BIN" log -1 --pretty=%B 2>/dev/null)"
                __devlog_capture_git_event "commit" "$REPO_PATH" "$BRANCH" \
                    --hash="$COMMIT_HASH" \
                    --message="$COMMIT_MESSAGE" \
                    --author="$COMMIT_AUTHOR"
            fi
        fi

        exit $EXIT_CODE
        ;;

    push)
        __devlog_get_repo_info
        PUSH_OUTPUT_FILE=$(mktemp)
        "$GIT_BIN" "$@" 2>&1 | tee "$PUSH_OUTPUT_FILE"
        EXIT_CODE=${PIPESTATUS[0]}

        if [ $EXIT_CODE -eq 0 ] && [ -n "$REPO_PATH" ]; then
            REMOTE="${2:-origin}"
            REF_SPEC="${3:-$BRANCH}"
            REMOTE_URL="$("$GIT_BIN" remote get-url "$REMOTE" 2>/dev/null)"

            COMMITS_PUSHED=$(grep -oE '[0-9a-f]{7}\.\.[0-9a-f]{7}' "$PUSH_OUTPUT_FILE" | wc -l | tr -d ' ')
            if [ -z "$COMMITS_PUSHED" ] || [ "$COMMITS_PUSHED" -eq 0 ]; then
                COMMITS_PUSHED=$(grep -oE 'new branch|branch.*->.*' "$PUSH_OUTPUT_FILE" | wc -l | tr -d ' ')
            fi

            __devlog_capture_git_event "push" "$REPO_PATH" "$BRANCH" \
                --remote="$REMOTE" \
                --ref="$REF_SPEC" \
                --remote-url="$REMOTE_URL" \
                --commits="$COMMITS_PUSHED"
        fi

        rm -f "$PUSH_OUTPUT_FILE"
        exit $EXIT_CODE
        ;;

    pull)
        __devlog_get_repo_info
        PULL_OUTPUT_FILE=$(mktemp)
        "$GIT_BIN" "$@" 2>&1 | tee "$PULL_OUTPUT_FILE"
        EXIT_CODE=${PIPESTATUS[0]}

        if [ $EXIT_CODE -eq 0 ] && [ -n "$REPO_PATH" ]; then
            REMOTE="${2:-origin}"
            REMOTE_URL="$("$GIT_BIN" remote get-url "$REMOTE" 2>/dev/null)"

            CHANGES="unknown"
            if grep -q "Fast-forward" "$PULL_OUTPUT_FILE"; then
                CHANGES="fast-forward"
            elif grep -q "Already up to date" "$PULL_OUTPUT_FILE"; then
                CHANGES="up-to-date"
            elif grep -q "Merge made" "$PULL_OUTPUT_FILE"; then
                CHANGES="merge"
            fi

            FILES_CHANGED=$(grep -E '^\s*[0-9]+ files? changed' "$PULL_OUTPUT_FILE" | grep -oE '[0-9]+' | head -1)

            __devlog_capture_git_event "pull" "$REPO_PATH" "$BRANCH" \
                --remote="$REMOTE" \
                --remote-url="$REMOTE_URL" \
                --changes="$CHANGES" \
                --files-changed="$FILES_CHANGED"
        fi

        rm -f "$PULL_OUTPUT_FILE"
        exit $EXIT_CODE
        ;;

    fetch)
        __devlog_get_repo_info
        FETCH_OUTPUT_FILE=$(mktemp)
        "$GIT_BIN" "$@" 2>&1 | tee "$FETCH_OUTPUT_FILE"
        EXIT_CODE=${PIPESTATUS[0]}

        if [ $EXIT_CODE -eq 0 ] && [ -n "$REPO_PATH" ]; then
            REMOTE="${2:-origin}"
            REMOTE_URL="$("$GIT_BIN" remote get-url "$REMOTE" 2>/dev/null)"

            REFS_UPDATED=$(grep -E '^\s*\*\s+\[|^\s+[0-9a-f]+\.\.[0-9a-f]+' "$FETCH_OUTPUT_FILE" | wc -l | tr -d ' ')

            __devlog_capture_git_event "fetch" "$REPO_PATH" "$BRANCH" \
                --remote="$REMOTE" \
                --remote-url="$REMOTE_URL" \
                --refs-updated="$REFS_UPDATED"
        fi

        rm -f "$FETCH_OUTPUT_FILE"
        exit $EXIT_CODE
        ;;

    merge)
        __devlog_get_repo_info
        "$GIT_BIN" "$@"
        EXIT_CODE=$?

        if [ $EXIT_CODE -eq 0 ] && [ -n "$REPO_PATH" ]; then
            MERGE_BRANCH="${2}"
            __devlog_capture_git_event "merge" "$REPO_PATH" "$BRANCH" \
                --merged-branch="$MERGE_BRANCH"
        fi

        exit $EXIT_CODE
        ;;

    rebase)
        __devlog_get_repo_info
        "$GIT_BIN" "$@"
        EXIT_CODE=$?

        if [ $EXIT_CODE -eq 0 ] && [ -n "$REPO_PATH" ]; then
            TARGET_BRANCH="${2}"
            __devlog_capture_git_event "rebase" "$REPO_PATH" "$BRANCH" \
                --target-branch="$TARGET_BRANCH"
        fi

        exit $EXIT_CODE
        ;;

    checkout|switch)
        __devlog_get_repo_info
        OLD_BRANCH="$BRANCH"
        "$GIT_BIN" "$@"
        EXIT_CODE=$?

        if [ $EXIT_CODE -eq 0 ] && [ -n "$REPO_PATH" ]; then
            NEW_BRANCH="$("$GIT_BIN" rev-parse --abbrev-ref HEAD 2>/dev/null)"
            [ "$NEW_BRANCH" = "HEAD" ] && NEW_BRANCH="detached-$("$GIT_BIN" rev-parse --short HEAD 2>/dev/null)"

            if [ "$OLD_BRANCH" != "$NEW_BRANCH" ]; then
                __devlog_capture_git_event "checkout" "$REPO_PATH" "$NEW_BRANCH" \
                    --from-branch="$OLD_BRANCH"
            fi
        fi

        exit $EXIT_CODE
        ;;

    stash)
        __devlog_get_repo_info
        STASH_SUBCOMMAND="${2:-push}"
        "$GIT_BIN" "$@"
        EXIT_CODE=$?

        if [ $EXIT_CODE -eq 0 ] && [ -n "$REPO_PATH" ]; then
            __devlog_capture_git_event "stash" "$REPO_PATH" "$BRANCH" \
                --stash-action="$STASH_SUBCOMMAND"
        fi

        exit $EXIT_CODE
        ;;

    *)
        exec "$GIT_BIN" "$@"
        ;;
esac
