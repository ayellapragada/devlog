# Plugins

Plugin implementations for devlog.

Plugins here add extra functionality, for exmple summarizing, or processing or exporting in some way.

## Overview

This directory contains concrete plugin implementations that extend devlog with additional functionality.

Plugins run as background processes in the daemon and can add AI-powered features, integrations, and automated workflows.

## Available Plugins

### [llm](./llm/README.md)

LLM client service provider.

**Features:**
- Provides shared LLM client to other plugins
- Supports multiple LLM providers (Anthropic, Ollama)
- Centralized configuration for AI services

### [summarizer](./summarizer/README.md)

AI-powered summarization plugin.

**Features:**
- Automatically generates natural language summaries of time intervals
- Clock-aligned scheduling for predictable summaries
- Configurable time windows and intervals

**Dependencies:** `llm`

## Plugin Architecture

All plugins in this directory:
1. Implement the `Plugin` interface from [internal/plugins](../internal/plugins/)
2. Declare metadata including dependencies via `Metadata()` method
3. Use `install.Context` for installation/uninstallation
4. Register themselves via `init()` function using `plugins.Register()`
5. Are imported in [internal/daemon/daemon.go](../internal/daemon/daemon.go) with blank imports
6. Run in isolated goroutines with individual contexts
7. Managed by daemon's plugin lifecycle system with dependency resolution
8. Support graceful shutdown and hot-reload

### Plugin Dependencies

Plugins can declare dependencies on other plugins:

```go
func (p *Plugin) Metadata() plugins.Metadata {
    return plugins.Metadata{
        Name:         "myplugin",
        Description:  "My awesome plugin",
        Dependencies: []string{"llm", "other-plugin"},
    }
}
```

The daemon automatically:
- Resolves dependencies using topological sort
- Starts plugins in dependency order
- Fails startup if dependencies are missing or circular
- Injects services from provider plugins

### Service Providers

Plugins can provide services to other plugins:

```go
func (p *Plugin) Services() map[string]interface{} {
    return map[string]interface{}{
        "myservice": p.serviceInstance,
    }
}
```

### Service Consumers

Plugins receive services via injection:

```go
func (p *Plugin) InjectServices(services map[string]interface{}) error {
    service, ok := services["myservice"]
    if !ok {
        return fmt.Errorf("required service not found")
    }
    p.service = service
    return nil
}
```

## Creating a New Plugin

### Directory Structure

```
plugins/
  yourplugin/
    plugin.go          # Plugin implementation
    README.md          # Plugin documentation
    *_test.go          # Tests
```

### Basic Template

```go
package yourplugin

import (
    "context"
    "fmt"
    "time"

    "devlog/internal/plugins"
)

type YourPlugin struct{}

func init() {
    plugins.Register(&YourPlugin{})
}

func (p *YourPlugin) Name() string {
    return "yourplugin"
}

func (p *YourPlugin) Description() string {
    return "Description of what your plugin does"
}

func (p *YourPlugin) Metadata() plugins.Metadata {
    return plugins.Metadata{
        Name:         "yourplugin",
        Description:  "Description of what your plugin does",
        Dependencies: []string{},  // Add dependencies here, e.g., []string{"llm"}
    }
}

func (p *YourPlugin) Install(ctx *install.Context) error {
    ctx.Log("Installing %s plugin...", p.Name())
    // Perform one-time setup
    return nil
}

func (p *YourPlugin) Uninstall(ctx *plugins.InstallContext) error {
    ctx.Log("Uninstalling %s plugin...", p.Name())
    // Cleanup
    return nil
}

func (p *YourPlugin) Start(ctx context.Context) error {
    // Get configuration from context
    // Run background process
    // Respect ctx.Done() for shutdown
    return nil
}

func (p *YourPlugin) DefaultConfig() interface{} {
    return map[string]interface{}{
        "enabled": true,
        // Add your config fields
    }
}

func (p *YourPlugin) ValidateConfig(config interface{}) error {
    // Validate configuration
    return nil
}
```

### Integration Checklist

- [ ] Implement all `Plugin` interface methods
- [ ] Use `common.InstallContext` for Install/Uninstall
- [ ] Register in `init()` using `plugins.Register()`
- [ ] Add blank import in [cmd/devlog/main.go](../cmd/devlog/main.go)
- [ ] Use standardized error wrappers from [internal/errors](../internal/errors/)
- [ ] Start() MUST return quickly (spawn goroutines for background work)
- [ ] Read config from context using `contextkeys.PluginConfig`
- [ ] Create comprehensive README with configuration docs
- [ ] Add tests
- [ ] Handle context cancellation gracefully in background goroutines
- [ ] Log errors using daemon logger

## Plugin Guidelines

### Configuration

Plugins should:
- Define sensible defaults in `DefaultConfig()` including `"enabled": false`
- Validate all configuration in `ValidateConfig()` before Start()
- Handle missing optional fields gracefully
- Read config from context: `ctx.Value(contextkeys.PluginConfig)`
- Document all configuration options in README with examples

### Error Handling

Plugins should:
- Return errors from `Install()` and `Uninstall()` for critical failures
- Use standardized error wrappers: `errors.WrapPlugin(pluginName, operation, err)`
- Log errors during `Start()` but don't crash the daemon
- Return quickly from Start() even if errors occur (log and continue in goroutine)
- Handle graceful degradation when external services unavailable
- Include context in error messages for debugging

### Resource Management

Plugins should:
- Clean up resources on context cancellation
- Use appropriate timeouts for external calls
- Close connections and files properly
- Respect shutdown timeouts (5 seconds)

### Performance

Plugins MUST:
- Return from Start() in < 1 second (spawn goroutines immediately)
- Not block daemon startup or other plugins

Plugins SHOULD:
- Use efficient polling intervals (balance freshness vs overhead)
- Batch operations when possible to reduce API calls
- Avoid excessive logging (use appropriate log levels)
- Implement backoff strategies for external service failures
- Respect context cancellation for quick shutdowns (< 5s timeout)

## Plugin Ideas

Potential future plugins:

### Analytics Plugin
- Track coding metrics over time
- Detect patterns and trends
- Generate insights

### Review Plugin
- AI-powered code review
- Suggest improvements
- Detect common issues

### Notification Plugin
- Send alerts for milestones
- Integrate with Slack/Discord
- Daily/weekly summaries

### Export Plugin
- Generate reports in various formats
- Export to external tools
- Data visualization

### Integration Plugin
- Sync with Jira/Linear/etc.
- Update task status automatically
- Link commits to tickets

### Learning Plugin
- Track learning progress
- Suggest resources
- Identify knowledge gaps

## Testing Plugins

Test your plugin thoroughly:

```go
func TestPlugin(t *testing.T) {
    p := &YourPlugin{}

    // Test Name and Description
    assert.NotEmpty(t, p.Name())
    assert.NotEmpty(t, p.Description())

    // Test DefaultConfig
    cfg := p.DefaultConfig()
    assert.NotNil(t, cfg)

    // Test ValidateConfig
    err := p.ValidateConfig(cfg)
    assert.NoError(t, err)

    // Test Start with cancellation
    ctx, cancel := context.WithCancel(context.Background())
    go func() {
        time.Sleep(100 * time.Millisecond)
        cancel()
    }()
    err = p.Start(ctx)
    assert.NoError(t, err)
}
```

## Documentation

Each plugin should have a README covering:
- What the plugin does
- Configuration options
- Installation/setup instructions
- Usage examples
- Troubleshooting
- Implementation details

See [summarizer/README.md](./summarizer/README.md) for a complete example.

## Contributing Plugins

When contributing a new plugin:

1. Follow the template structure
2. Write comprehensive tests
3. Document all configuration
4. Handle errors gracefully
5. Add example configuration
6. Update this README
7. Test installation and uninstallation

## Plugin System Benefits

**Extensibility**: Add features without modifying core code

**Modularity**: Enable/disable features independently

**Isolation**: Plugin failures don't crash daemon

**Reusability**: Share plugins across installations

**Flexibility**: Support multiple implementations (e.g., different LLM providers)

## See Also

- [Plugin Interface Documentation](../internal/plugins/README.md)
- [Daemon Architecture](../internal/daemon/README.md)
- [Configuration Guide](../internal/config/README.md)
