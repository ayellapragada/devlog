#!/bin/bash
# Install devlog git hooks globally for all repositories

set -e

echo "Installing devlog git hooks globally..."
echo

# Create global hooks directory
HOOKS_DIR="$HOME/.config/git/hooks"
mkdir -p "$HOOKS_DIR"

# Get the source hook path
HOOK_SOURCE="$(cd "$(dirname "$0")" && pwd)/hooks/post-commit"

if [ ! -f "$HOOK_SOURCE" ]; then
    echo "Error: post-commit hook not found at $HOOK_SOURCE"
    exit 1
fi

# Check if global hooks are already configured
CURRENT_HOOKS_PATH=$(git config --global --get core.hooksPath || echo "")

if [ -n "$CURRENT_HOOKS_PATH" ] && [ "$CURRENT_HOOKS_PATH" != "$HOOKS_DIR" ]; then
    echo "Warning: Git is already configured to use a different global hooks directory:"
    echo "  $CURRENT_HOOKS_PATH"
    echo
    read -p "Switch to $HOOKS_DIR? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Installation cancelled"
        echo
        echo "To install manually, copy the hook and configure git:"
        echo "  cp $HOOK_SOURCE $CURRENT_HOOKS_PATH/"
        echo "  chmod +x $CURRENT_HOOKS_PATH/post-commit"
        exit 0
    fi
fi

# Copy hook
HOOK_DEST="$HOOKS_DIR/post-commit"

if [ -f "$HOOK_DEST" ]; then
    echo "Warning: post-commit hook already exists at $HOOK_DEST"
    read -p "Overwrite? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Installation cancelled"
        exit 0
    fi
fi

cp "$HOOK_SOURCE" "$HOOK_DEST"
chmod +x "$HOOK_DEST"

# Configure git to use global hooks directory
git config --global core.hooksPath "$HOOKS_DIR"

echo "✓ Installed post-commit hook to $HOOK_DEST"
echo "✓ Configured git to use global hooks directory: $HOOKS_DIR"
echo
echo "All git repositories on this system will now send commit events to devlog."
echo "Make sure:"
echo "  1. devlog is in your PATH, or"
echo "  2. Update the hook to use an absolute path to devlog"
