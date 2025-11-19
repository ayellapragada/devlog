#!/bin/bash

DEVLOG_KUBECTL_ENABLED="${DEVLOG_KUBECTL_ENABLED:-true}"

find_real_kubectl() {
    local this_script="$(realpath "${BASH_SOURCE[0]}" 2>/dev/null || readlink -f "${BASH_SOURCE[0]}" 2>/dev/null)"
    [ -z "$this_script" ] && this_script="${BASH_SOURCE[0]}"

    IFS=: read -ra paths <<< "$PATH"
    for dir in "${paths[@]}"; do
        [ -z "$dir" ] && continue
        local candidate="$dir/kubectl"
        [ ! -x "$candidate" ] && continue
        local candidate_real="$(realpath "$candidate" 2>/dev/null || readlink -f "$candidate" 2>/dev/null)"
        [ -z "$candidate_real" ] && candidate_real="$candidate"
        [ "$candidate_real" = "$this_script" ] && continue
        echo "$candidate"
        return 0
    done

    if command -v kubectl &> /dev/null; then
        command -v kubectl
        return 0
    fi

    echo "/usr/local/bin/kubectl"
}

KUBECTL_BIN="$(find_real_kubectl)"
[ "$DEVLOG_KUBECTL_ENABLED" != "true" ] && exec "$KUBECTL_BIN" "$@"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMMON_LIB="${SCRIPT_DIR}/devlog-kubectl-common.sh"

if [ -f "$COMMON_LIB" ]; then
    source "$COMMON_LIB"
elif [ -f "${HOME}/.local/bin/devlog-kubectl-common.sh" ]; then
    source "${HOME}/.local/bin/devlog-kubectl-common.sh"
else
    exec "$KUBECTL_BIN" "$@"
fi

SUBCOMMAND="$1"

case "$SUBCOMMAND" in
    apply|create)
        __devlog_get_kubectl_context
        OUTPUT_FILE=$(mktemp)
        "$KUBECTL_BIN" "$@" 2>&1 | tee "$OUTPUT_FILE"
        EXIT_CODE=${PIPESTATUS[0]}

        if [ -n "$KUBECTL_CONTEXT" ]; then
            NAMESPACE=$(__devlog_extract_namespace "$@")
            RESOURCE_TYPE=$(__devlog_extract_resource_type "$@")
            RESOURCE_NAMES=$(grep -oE '(created|configured|unchanged)$' "$OUTPUT_FILE" | wc -l | tr -d ' ')

            __devlog_capture_kubectl_event "$SUBCOMMAND" "$KUBECTL_CONTEXT" "$NAMESPACE" \
                --resource-type="$RESOURCE_TYPE" \
                --resource-count="$RESOURCE_NAMES" \
                --exit-code="$EXIT_CODE" &
        fi

        rm -f "$OUTPUT_FILE"
        exit $EXIT_CODE
        ;;

    delete)
        __devlog_get_kubectl_context
        OUTPUT_FILE=$(mktemp)
        "$KUBECTL_BIN" "$@" 2>&1 | tee "$OUTPUT_FILE"
        EXIT_CODE=${PIPESTATUS[0]}

        if [ -n "$KUBECTL_CONTEXT" ]; then
            NAMESPACE=$(__devlog_extract_namespace "$@")
            RESOURCE_TYPE=$(__devlog_extract_resource_type "$@")
            RESOURCE_NAMES=$(__devlog_extract_resource_names "$@")

            __devlog_capture_kubectl_event "delete" "$KUBECTL_CONTEXT" "$NAMESPACE" \
                --resource-type="$RESOURCE_TYPE" \
                --resource-names="$RESOURCE_NAMES" \
                --exit-code="$EXIT_CODE" &
        fi

        rm -f "$OUTPUT_FILE"
        exit $EXIT_CODE
        ;;

    get|describe)
        __devlog_get_kubectl_context
        "$KUBECTL_BIN" "$@"
        EXIT_CODE=$?

        if [ -n "$KUBECTL_CONTEXT" ]; then
            NAMESPACE=$(__devlog_extract_namespace "$@")
            RESOURCE_TYPE=$(__devlog_extract_resource_type "$@")
            RESOURCE_NAMES=$(__devlog_extract_resource_names "$@")

            __devlog_capture_kubectl_event "$SUBCOMMAND" "$KUBECTL_CONTEXT" "$NAMESPACE" \
                --resource-type="$RESOURCE_TYPE" \
                --resource-names="$RESOURCE_NAMES" \
                --exit-code="$EXIT_CODE" &
        fi

        exit $EXIT_CODE
        ;;

    edit|patch)
        __devlog_get_kubectl_context
        "$KUBECTL_BIN" "$@"
        EXIT_CODE=$?

        if [ $EXIT_CODE -eq 0 ] && [ -n "$KUBECTL_CONTEXT" ]; then
            NAMESPACE=$(__devlog_extract_namespace "$@")
            RESOURCE_TYPE=$(__devlog_extract_resource_type "$@")
            RESOURCE_NAMES=$(__devlog_extract_resource_names "$@")

            __devlog_capture_kubectl_event "$SUBCOMMAND" "$KUBECTL_CONTEXT" "$NAMESPACE" \
                --resource-type="$RESOURCE_TYPE" \
                --resource-names="$RESOURCE_NAMES" \
                --exit-code="$EXIT_CODE" &
        fi

        exit $EXIT_CODE
        ;;

    logs|exec|debug)
        __devlog_get_kubectl_context
        "$KUBECTL_BIN" "$@"
        EXIT_CODE=$?

        if [ -n "$KUBECTL_CONTEXT" ]; then
            NAMESPACE=$(__devlog_extract_namespace "$@")
            RESOURCE_TYPE=$(__devlog_extract_resource_type "$@")
            RESOURCE_NAMES=$(__devlog_extract_resource_names "$@")

            __devlog_capture_kubectl_event "$SUBCOMMAND" "$KUBECTL_CONTEXT" "$NAMESPACE" \
                --resource-type="$RESOURCE_TYPE" \
                --resource-names="$RESOURCE_NAMES" \
                --exit-code="$EXIT_CODE" &
        fi

        exit $EXIT_CODE
        ;;

    *)
        exec "$KUBECTL_BIN" "$@"
        ;;
esac
