# Claude Code Instructions

## Workflow Guidelines

- Only commit if explicitly prompted, otherwise leave git operations to the user.
- We do not care about backwards compatability, the app is in early stages, breaking changes are expected.

## Code Style Guidelines

### Comments Policy
- **DO NOT** add explanatory comments to code unless explicitly requested by the user
- **DO NOT** add inline comments explaining what code does
- **DO** add comments only when the user explicitly asks for them, or documentation
- **DO** add comments for complex algorithms where a brief explanation adds significant value
- Keep the codebase clean and comment-free unless comments already exist or are specifically requested
- Trust that the code is self-documenting through clear naming and structure

### Error Handling Guidelines

All errors should use standardized error wrappers from `internal/errors` for consistency and better debugging.

#### Available Error Wrappers

```go
import "devlog/internal/errors"

// Storage operations
errors.WrapStorage(operation, err)           // "storage {operation} failed: {err}"

// Module operations
errors.WrapModule(module, operation, err)    // "module {module}: {operation} failed: {err}"

// Module installation (with recovery steps)
errors.WrapInstall(component, file, err, steps...)

// Daemon operations
errors.WrapDaemon(component, err)            // "daemon {component}: {err}"

// Plugin operations
errors.WrapPlugin(plugin, operation, err)    // "plugin {plugin}: {operation} failed: {err}"

// Queue operations
errors.WrapQueue(operation, err)             // "queue {operation}: {err}"

// Validation errors
errors.NewValidation(field, message)         // "validation failed for {field}: {message}"
```

#### Usage Guidelines

1. **Use wrapper functions instead of fmt.Errorf**
   ```go
   // ❌ Bad
   return fmt.Errorf("open database: %w", err)

   // ✅ Good
   return errors.WrapStorage("open database", err)
   ```

2. **Choose the right wrapper for the layer**
   - Storage layer → `WrapStorage`
   - Module operations → `WrapModule`
   - Daemon operations → `WrapDaemon`
   - Plugin operations → `WrapPlugin`
   - Queue operations → `WrapQueue`
   - Validation → `NewValidation`

3. **Use descriptive operation names**
   ```go
   // ✅ Good: Clear what failed
   errors.WrapStorage("enable WAL mode", err)
   errors.WrapQueue("serialize event", err)
   errors.WrapDaemon("start services", err)
   ```

4. **Installation errors need recovery steps**
   ```go
   return errors.WrapInstall(
       "git wrapper",
       wrapperPath,
       err,
       "Check file permissions: ls -la " + filepath.Dir(wrapperPath),
       "Ensure directory exists and is writable",
       "Try manual install: chmod +x " + wrapperPath,
   )
   ```

5. **All error wrappers return nil if err is nil**
   ```go
   // Safe to use without checking if err is nil
   return errors.WrapStorage("query", err)  // Returns nil if err is nil
   ```

6. **User-facing errors can still use fmt.Errorf**
   ```go
   // ✅ OK: Informative user message without underlying error
   return fmt.Errorf("event not found: %s", id)
   return fmt.Errorf("database already exists at %s", dbPath)
   ```

#### Error Context Philosophy

- Errors should be **wrapped at the layer boundary** where they occur
- Each wrapper adds **one level of context** about where/what failed
- The original error is preserved through `Unwrap()` for debugging
- Error messages should be **actionable** - tell users what went wrong and ideally how to fix it

## Plugin/Module Contract

### Error Handling Philosophy
- Plugins should gracefully degrade on errors, not crash the daemon
- Errors should be logged with context but not stop other plugins
- Modules should fail installation if critical steps fail, but daemon continues

### Plugin Lifecycle

#### Start() Method Requirements
- MUST return quickly (< 5 seconds)
- Should spawn goroutines for background work
- MUST respect context cancellation
- Return error only for fatal initialization failures

Example:
```go
func (p *MyPlugin) Start(ctx context.Context) error {
    // ✅ Good: Spawn background work
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

// ❌ Bad: Blocks daemon startup
func (p *BadPlugin) Start(ctx context.Context) error {
    for {
        p.doWork()
        time.Sleep(30 * time.Second)
    }
}
```

#### Concurrency Model
- Each plugin runs independently in its own goroutine
- Plugins cannot block each other
- Shared resources (storage, config) are thread-safe
- Plugins receive their own context for cancellation

#### Configuration
- Config is passed explicitly, not through context values
- Validate config in ValidateConfig(), not Start()
- Invalid config should fail validation, not crash plugin

### Module Installation

#### Installation Requirements
- MUST be idempotent (safe to run multiple times)
- MUST check if already installed before modifying files
- MUST provide clear error messages with recovery steps
- SHOULD backup files before modifying

#### Error Messages
Include:
- What failed
- Why it failed
- How to fix it manually
- Relevant file paths

Example:
```go
return fmt.Errorf(`Failed to install shell integration:
  File: %s
  Error: %w

  To fix:
  1. Check file permissions: ls -la %s
  2. Ensure directory exists: mkdir -p %s
  3. Try manual install: devlog install shell --verbose`,
  bashrcPath, err, bashrcPath, filepath.Dir(bashrcPath))
```