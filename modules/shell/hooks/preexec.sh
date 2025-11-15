#!/bin/bash
# devlog preexec hook
# Captures shell commands before execution
# This script is meant to be sourced in your shell's rc file

# Store command start time
export DEVLOG_CMD_START=$(date +%s%3N)
export DEVLOG_CMD="$1"
