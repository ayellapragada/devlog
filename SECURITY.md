# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in DevLog, please report it by emailing the maintainers directly. Please **do not** open a public GitHub issue for security vulnerabilities.

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### What to Expect

- Acknowledgment within 48 hours
- Regular updates on the status of your report
- Credit in the security advisory (if desired)

## Security Considerations

### Local Data Storage

DevLog stores all data locally in a SQLite database at `~/.devlog/devlog.db`. This includes:
- Git commit messages and diffs
- Shell command history
- Code snippets from clipboard
- LLM prompts and responses

**Important**:
- Never share your DevLog database publicly
- Be cautious when using cloud sync services (Dropbox, iCloud, etc.) with `~/.devlog/`
- Review generated summaries before sharing them

### LLM Provider Configuration

DevLog supports multiple LLM providers (OpenAI, Anthropic, Gemini, Ollama). API keys are stored in:
- Configuration file: `~/.devlog/config.yaml`
- Environment variables

**Recommendations**:
- Use environment variables for API keys when possible
- Ensure config files have appropriate permissions (600)
- Never commit API keys to version control
- Use provider-specific rate limiting to prevent unexpected costs

### Daemon Security

The DevLog daemon runs as a background process. Security considerations:
- Runs under your user account (not as root)
- Listens only on localhost (no network exposure)
- No authentication required (single-user system)

### Module Installation

DevLog modules can modify shell configuration files (`.bashrc`, `.zshrc`, etc.) during installation:
- Always review installation changes with `--dry-run` first
- Backups are created before modifications
- Uninstall cleanly removes all changes

## Best Practices

1. **Review captured data periodically**: `devlog events list`
2. **Keep DevLog updated**: Security fixes are released promptly
3. **Secure your API keys**: Use environment variables or secure credential storage
4. **Limit LLM data sharing**: Configure which events are sent to LLM providers
5. **Use Ollama for sensitive work**: Run LLMs locally to keep data offline

## Known Limitations

- DevLog does not encrypt data at rest (SQLite database is plaintext)
- No built-in data redaction for secrets in git commits or commands
- Shell history capture may include sensitive commands

If you need enhanced security, consider:
- Using disk encryption for `~/.devlog/`
- Running Ollama locally instead of cloud LLM providers
- Disabling specific modules that capture sensitive data
