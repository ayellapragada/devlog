# Contributing to DevLog

Thanks for your interest in contributing to DevLog! This guide will help you get started.

## Development Setup

### Prerequisites

- Go 1.25 or later
- Unix-like OS (macOS or Linux)
- Git

### Getting Started

1. **Fork and clone the repository**
   ```bash
   git clone https://github.com/YOUR_USERNAME/devlog.git
   cd devlog
   ```

2. **Build the project**
   ```bash
   make build
   ```

3. **Run tests**
   ```bash
   make test
   ```

4. **Install locally for testing**
   ```bash
   ./devlog install
   ```

## Project Structure

```
devlog/
├── cmd/              # CLI entry points
│   └── devlog/       # Main CLI application
├── internal/         # Core internal packages
│   ├── daemon/       # Background daemon service
│   ├── storage/      # SQLite database layer
│   ├── config/       # Configuration management
│   └── ...
├── modules/          # Event capture modules
│   ├── git/          # Git hook integration
│   ├── shell/        # Shell history capture
│   └── ...
└── plugins/          # Event processing plugins
    ├── summarizer/   # LLM-based summarization
    └── ...
```

## Code Style

### General Guidelines

- **No unnecessary comments**: Code should be self-documenting through clear naming
- **Error handling**: Use standardized error wrappers from `internal/errors`
- **Breaking changes**: Expected and acceptable for early versions
- **Testing**: Add tests for new functionality

### Error Handling

Use the error wrappers from [internal/errors](internal/errors):

```go
// Storage operations
errors.WrapStorage("operation", err)

// Module operations
errors.WrapModule("module", "operation", err)

// Daemon operations
errors.WrapDaemon("component", err)

// Plugin operations
errors.WrapPlugin("plugin", "operation", err)

// Validation errors
errors.NewValidation("field", "message")
```

### Plugin Development

Plugins must:
- Return quickly from `Start()` (< 5 seconds)
- Spawn goroutines for background work
- Respect context cancellation
- Handle errors gracefully without crashing the daemon

Example:
```go
func (p *MyPlugin) Start(ctx context.Context) error {
    go p.run(ctx)
    return nil
}

func (p *MyPlugin) run(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            p.doWork()
        }
    }
}
```

### Module Development

Modules must:
- Be idempotent (safe to run multiple times)
- Check if already installed before modifying files
- Provide clear error messages with recovery steps
- Back up files before modifying

## Making Changes

1. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes**
   - Write tests for new functionality
   - Ensure all tests pass: `make test`
   - Follow the code style guidelines

3. **Commit your changes**
   ```bash
   git commit -m "feat: add new feature"
   ```

   Use conventional commit messages:
   - `feat:` - New features
   - `fix:` - Bug fixes
   - `docs:` - Documentation changes
   - `refactor:` - Code refactoring
   - `test:` - Adding tests
   - `chore:` - Maintenance tasks

4. **Push and create a pull request**
   ```bash
   git push origin feature/your-feature-name
   ```

## Pull Request Process

1. Update documentation if needed
2. Add tests for new functionality
3. Ensure CI passes
4. Request review from maintainers
5. Address any feedback

## Testing

Run the full test suite:
```bash
make test
```

Run tests with detailed coverage:
```bash
make test-verbose
```

Run tests for a specific package:
```bash
go test ./internal/daemon
```

Run all checks (format, lint, test):
```bash
make check
```

## Questions?

- Open an issue for bugs or feature requests
- Start a discussion for general questions
- Check existing issues before creating new ones

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
