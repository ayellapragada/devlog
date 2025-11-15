#!/bin/bash
# devlog shell hook installer
# Installs devlog shell hooks for bash and zsh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK_SCRIPT="$SCRIPT_DIR/devlog.sh"

# Color output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

error() {
    echo -e "${RED}Error: $1${NC}" >&2
    exit 1
}

info() {
    echo -e "${GREEN}$1${NC}"
}

warn() {
    echo -e "${YELLOW}$1${NC}"
}

# Check if hook script exists
if [ ! -f "$HOOK_SCRIPT" ]; then
    error "Hook script not found at $HOOK_SCRIPT"
fi

# Detect current shell
CURRENT_SHELL=$(basename "$SHELL")

info "DevLog Shell Hook Installer"
info "============================"
echo ""
info "Current shell: $CURRENT_SHELL"
echo ""

# Function to add source line to rc file
add_to_rc() {
    local rc_file="$1"
    local source_line="source \"$HOOK_SCRIPT\""

    # Check if already installed
    if [ -f "$rc_file" ] && grep -q "devlog.sh" "$rc_file"; then
        warn "Already installed in $rc_file"
        return 0
    fi

    # Backup rc file if it exists
    if [ -f "$rc_file" ]; then
        cp "$rc_file" "$rc_file.backup.$(date +%s)"
        info "Created backup: $rc_file.backup.$(date +%s)"
    fi

    # Add source line
    echo "" >> "$rc_file"
    echo "# devlog shell integration" >> "$rc_file"
    echo "$source_line" >> "$rc_file"

    info "Added devlog hook to $rc_file"
}

# Install for bash
install_bash() {
    info "Installing for Bash..."
    local bashrc="$HOME/.bashrc"

    # On macOS, also check .bash_profile
    if [[ "$OSTYPE" == "darwin"* ]]; then
        if [ -f "$HOME/.bash_profile" ]; then
            add_to_rc "$HOME/.bash_profile"
        else
            add_to_rc "$bashrc"
        fi
    else
        add_to_rc "$bashrc"
    fi
}

# Install for zsh
install_zsh() {
    info "Installing for Zsh..."
    local zshrc="$HOME/.zshrc"
    add_to_rc "$zshrc"
}

# Main installation logic
case "$CURRENT_SHELL" in
    bash)
        install_bash
        ;;
    zsh)
        install_zsh
        ;;
    *)
        echo ""
        warn "Unsupported shell: $CURRENT_SHELL"
        warn "Please manually add the following line to your shell's RC file:"
        echo ""
        echo "  source \"$HOOK_SCRIPT\""
        echo ""
        exit 1
        ;;
esac

echo ""
info "Installation complete!"
echo ""
echo "To activate the hooks:"
echo "  1. Restart your shell, or"
echo "  2. Run: source ~/${CURRENT_SHELL}rc"
echo ""
echo "Configuration:"
echo "  - Shell hooks can be configured in ~/.config/devlog/config.yaml"
echo "  - Set 'shell.enabled: false' to disable"
echo "  - Customize 'shell.ignore_list' to filter commands"
echo ""
echo "To uninstall, remove the 'devlog shell integration' section from your RC file."
