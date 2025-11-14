#!/bin/bash
# Install devlog git hooks into a repository

set -e

if [ $# -eq 0 ]; then
    echo "Usage: $0 <path-to-git-repo>"
    echo
    echo "Example:"
    echo "  $0 ~/dev/myproject"
    echo
    echo "This will install the devlog post-commit hook into the specified repository."
    exit 1
fi

REPO_PATH="$1"

# Check if it's a git repository
if [ ! -d "$REPO_PATH/.git" ]; then
    echo "Error: $REPO_PATH is not a git repository"
    exit 1
fi

# Get the hooks directory
HOOKS_DIR="$REPO_PATH/.git/hooks"

# Copy post-commit hook
HOOK_SOURCE="$(cd "$(dirname "$0")" && pwd)/post-commit"
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

echo "✓ Installed post-commit hook to $HOOK_DEST"
echo
echo "The hook will now capture git commits and send them to devlogd."
echo "Make sure devlog is in your PATH or update the hook to use an absolute path."
