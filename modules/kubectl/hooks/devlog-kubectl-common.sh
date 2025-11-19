#!/bin/bash

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

__devlog_get_kubectl_context() {
    KUBECTL_CONTEXT=$("$KUBECTL_BIN" config current-context 2>/dev/null)
    KUBECTL_CLUSTER=$("$KUBECTL_BIN" config view --minify -o jsonpath='{.clusters[0].name}' 2>/dev/null)
}

__devlog_extract_namespace() {
    local args=("$@")
    local namespace=""

    for i in "${!args[@]}"; do
        if [[ "${args[$i]}" == "-n" ]] || [[ "${args[$i]}" == "--namespace" ]]; then
            namespace="${args[$((i+1))]}"
            break
        elif [[ "${args[$i]}" == --namespace=* ]]; then
            namespace="${args[$i]#*=}"
            break
        fi
    done

    if [ -z "$namespace" ]; then
        namespace=$("$KUBECTL_BIN" config view --minify -o jsonpath='{.contexts[0].context.namespace}' 2>/dev/null)
    fi

    if [ -z "$namespace" ]; then
        namespace="default"
    fi

    echo "$namespace"
}

__devlog_extract_resource_type() {
    local args=("$@")

    for arg in "${args[@]}"; do
        if [[ ! "$arg" =~ ^- ]] && [[ "$arg" != "$1" ]]; then
            echo "$arg"
            return
        fi
    done

    echo "unknown"
}

__devlog_extract_resource_names() {
    local args=("$@")
    local resource_type=""
    local resource_names=()
    local skip_next=false

    for i in "${!args[@]}"; do
        if [ "$skip_next" = true ]; then
            skip_next=false
            continue
        fi

        local arg="${args[$i]}"

        if [[ "$arg" == "-n" ]] || [[ "$arg" == "--namespace" ]] || \
           [[ "$arg" == "-f" ]] || [[ "$arg" == "--filename" ]] || \
           [[ "$arg" == "-o" ]] || [[ "$arg" == "--output" ]] || \
           [[ "$arg" == "-l" ]] || [[ "$arg" == "--selector" ]]; then
            skip_next=true
            continue
        fi

        if [[ "$arg" =~ ^- ]] || [[ "$arg" == "${args[0]}" ]]; then
            continue
        fi

        if [ -z "$resource_type" ]; then
            resource_type="$arg"
        else
            resource_names+=("$arg")
        fi
    done

    if [ ${#resource_names[@]} -eq 0 ]; then
        echo "all"
    else
        echo "${resource_names[*]}"
    fi
}

__devlog_capture_kubectl_event() {
    [ -z "$DEVLOG_BIN_PATH" ] && return

    local operation="$1"
    local context="$2"
    local namespace="$3"
    shift 3

    "$DEVLOG_BIN_PATH" ingest kubectl \
        --operation="$operation" \
        --context="$context" \
        --cluster="$KUBECTL_CLUSTER" \
        --namespace="$namespace" \
        "$@" &> /dev/null
}
