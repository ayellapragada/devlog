#!/bin/bash

DEVLOG_GIT_ENABLED="${DEVLOG_GIT_ENABLED:-true}"
[ "$DEVLOG_GIT_ENABLED" != "true" ] && exec /usr/bin/git "$@"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMMON_LIB="${SCRIPT_DIR}/devlog-git-common.sh"

if [ -f "$COMMON_LIB" ]; then
    source "$COMMON_LIB"
elif [ -f "${HOME}/.local/bin/devlog-git-common.sh" ]; then
    source "${HOME}/.local/bin/devlog-git-common.sh"
else
    exec /usr/bin/git "$@"
fi

GIT_BIN="/usr/bin/git"
SUBCOMMAND="$1"

case "$SUBCOMMAND" in
    commit)
        __devlog_get_repo_info
        "$GIT_BIN" "$@"
        EXIT_CODE=$?

        if [ $EXIT_CODE -eq 0 ] && [ -n "$REPO_PATH" ]; then
            read -r COMMIT_HASH COMMIT_AUTHOR < <(/usr/bin/git log -1 --format='%H %an' 2>/dev/null)
            if [ -n "$COMMIT_HASH" ]; then
                COMMIT_MESSAGE="$(/usr/bin/git log -1 --pretty=%B 2>/dev/null)"
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
        "$GIT_BIN" "$@"
        EXIT_CODE=$?

        if [ $EXIT_CODE -eq 0 ] && [ -n "$REPO_PATH" ]; then
            REMOTE="${2:-origin}"
            REF_SPEC="${3:-$BRANCH}"
            __devlog_capture_git_event "push" "$REPO_PATH" "$BRANCH" \
                --remote="$REMOTE" \
                --ref="$REF_SPEC"
        fi

        exit $EXIT_CODE
        ;;

    pull)
        __devlog_get_repo_info
        "$GIT_BIN" "$@"
        EXIT_CODE=$?

        if [ $EXIT_CODE -eq 0 ] && [ -n "$REPO_PATH" ]; then
            REMOTE="${2:-origin}"
            __devlog_capture_git_event "pull" "$REPO_PATH" "$BRANCH" \
                --remote="$REMOTE"
        fi

        exit $EXIT_CODE
        ;;

    fetch)
        __devlog_get_repo_info
        "$GIT_BIN" "$@"
        EXIT_CODE=$?

        if [ $EXIT_CODE -eq 0 ] && [ -n "$REPO_PATH" ]; then
            REMOTE="${2:-origin}"
            __devlog_capture_git_event "fetch" "$REPO_PATH" "$BRANCH" \
                --remote="$REMOTE"
        fi

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
            NEW_BRANCH="$(/usr/bin/git rev-parse --abbrev-ref HEAD 2>/dev/null)"
            [ "$NEW_BRANCH" = "HEAD" ] && NEW_BRANCH="detached-$(/usr/bin/git rev-parse --short HEAD 2>/dev/null)"

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
