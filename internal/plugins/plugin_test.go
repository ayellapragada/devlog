package plugins

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"devlog/internal/install"
)

type testPlugin struct {
	mu      sync.Mutex
	started bool
	stopped bool
}

func (p *testPlugin) Name() string {
	return "test"
}

func (p *testPlugin) Description() string {
	return "Test plugin"
}

func (p *testPlugin) Install(ctx *install.Context) error {
	return nil
}

func (p *testPlugin) Uninstall(ctx *install.Context) error {
	return nil
}

func (p *testPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	p.started = true
	p.mu.Unlock()
	<-ctx.Done()
	p.mu.Lock()
	p.stopped = true
	p.mu.Unlock()
	return nil
}

func (p *testPlugin) DefaultConfig() interface{} {
	return map[string]interface{}{}
}

func (p *testPlugin) ValidateConfig(config interface{}) error {
	return nil
}

func (p *testPlugin) Metadata() Metadata {
	return Metadata{
		Name:         p.Name(),
		Description:  p.Description(),
		Dependencies: []string{},
	}
}

func TestPluginRegistry(t *testing.T) {
	mu.Lock()
	plugins = make(map[string]Plugin)
	mu.Unlock()

	plugin := &testPlugin{}

	if err := Register(plugin); err != nil {
		t.Fatalf("Register() error: %v", err)
	}

	retrieved, err := Get("test")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	if retrieved.Name() != "test" {
		t.Errorf("got name %s, want test", retrieved.Name())
	}

	// List plugins
	plugins := List()
	if len(plugins) != 1 {
		t.Errorf("got %d plugins, want 1", len(plugins))
	}
}

func TestPluginLifecycle(t *testing.T) {
	plugin := &testPlugin{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- plugin.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	plugin.mu.Lock()
	started := plugin.started
	plugin.mu.Unlock()
	if !started {
		t.Error("plugin did not start")
	}

	cancel()

	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("plugin.Start() error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("plugin did not stop")
	}

	plugin.mu.Lock()
	stopped := plugin.stopped
	plugin.mu.Unlock()
	if !stopped {
		t.Error("plugin did not stop")
	}
}

type failingPlugin struct {
	failOn string
}

func (p *failingPlugin) Name() string {
	return "failing"
}

func (p *failingPlugin) Description() string {
	return "Plugin that fails"
}

func (p *failingPlugin) Install(ctx *install.Context) error {
	if p.failOn == "install" {
		return fmt.Errorf("install failed")
	}
	return nil
}

func (p *failingPlugin) Uninstall(ctx *install.Context) error {
	if p.failOn == "uninstall" {
		return fmt.Errorf("uninstall failed")
	}
	return nil
}

func (p *failingPlugin) Start(ctx context.Context) error {
	if p.failOn == "start" {
		return fmt.Errorf("start failed")
	}
	<-ctx.Done()
	return nil
}

func (p *failingPlugin) DefaultConfig() interface{} {
	return map[string]interface{}{}
}

func (p *failingPlugin) ValidateConfig(config interface{}) error {
	if p.failOn == "validate" {
		return fmt.Errorf("validation failed")
	}
	return nil
}

func TestPluginStartFailure(t *testing.T) {
	plugin := &failingPlugin{failOn: "start"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := plugin.Start(ctx)
	if err == nil {
		t.Error("Start() should fail when plugin returns error")
	}

	if err.Error() != "start failed" {
		t.Errorf("Expected 'start failed', got '%s'", err.Error())
	}
}

func TestPluginInstallFailure(t *testing.T) {
	plugin := &failingPlugin{failOn: "install"}

	ctx := &install.Context{}
	err := plugin.Install(ctx)
	if err == nil {
		t.Error("Install() should fail when plugin returns error")
	}

	if err.Error() != "install failed" {
		t.Errorf("Expected 'install failed', got '%s'", err.Error())
	}
}

func TestPluginUninstallFailure(t *testing.T) {
	plugin := &failingPlugin{failOn: "uninstall"}

	ctx := &install.Context{}
	err := plugin.Uninstall(ctx)
	if err == nil {
		t.Error("Uninstall() should fail when plugin returns error")
	}

	if err.Error() != "uninstall failed" {
		t.Errorf("Expected 'uninstall failed', got '%s'", err.Error())
	}
}

func TestPluginValidateConfigFailure(t *testing.T) {
	plugin := &failingPlugin{failOn: "validate"}

	err := plugin.ValidateConfig(nil)
	if err == nil {
		t.Error("ValidateConfig() should fail when plugin returns error")
	}

	if err.Error() != "validation failed" {
		t.Errorf("Expected 'validation failed', got '%s'", err.Error())
	}
}

func TestPluginContextCancellation(t *testing.T) {
	plugin := &testPlugin{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := plugin.Start(ctx)
	if err != nil {
		t.Errorf("Start() should not return error on context cancellation: %v", err)
	}

	plugin.mu.Lock()
	stopped := plugin.stopped
	plugin.mu.Unlock()
	if !stopped {
		t.Error("plugin should stop on context cancellation")
	}
}

func TestDuplicatePluginRegistration(t *testing.T) {
	mu.Lock()
	plugins = make(map[string]Plugin)
	mu.Unlock()

	plugin1 := &testPlugin{}
	plugin2 := &testPlugin{}

	if err := Register(plugin1); err != nil {
		t.Fatalf("First Register() should succeed: %v", err)
	}

	err := Register(plugin2)
	if err == nil {
		t.Error("Second Register() should fail for duplicate plugin name")
	}

	if err != nil && err.Error() != "plugin test is already registered" {
		t.Errorf("Expected duplicate registration error, got: %s", err.Error())
	}
}

func TestGetNonExistentPlugin(t *testing.T) {
	mu.Lock()
	plugins = make(map[string]Plugin)
	mu.Unlock()

	_, err := Get("nonexistent")
	if err == nil {
		t.Error("Get() should fail for non-existent plugin")
	}

	if err != nil && err.Error() != "plugin nonexistent not found" {
		t.Errorf("Expected not found error, got: %s", err.Error())
	}
}

type panicPlugin struct{}

func (p *panicPlugin) Name() string {
	return "panic"
}

func (p *panicPlugin) Description() string {
	return "Plugin that panics"
}

func (p *panicPlugin) Install(ctx *install.Context) error {
	panic("install panic")
}

func (p *panicPlugin) Uninstall(ctx *install.Context) error {
	panic("uninstall panic")
}

func (p *panicPlugin) Start(ctx context.Context) error {
	panic("start panic")
}

func (p *panicPlugin) DefaultConfig() interface{} {
	return nil
}

func (p *panicPlugin) ValidateConfig(config interface{}) error {
	panic("validate panic")
}

func TestPluginPanicRecovery(t *testing.T) {
	plugin := &panicPlugin{}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic to be propagated")
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = plugin.Start(ctx)
}

type namedTestPlugin struct {
	customName string
}

func (p *namedTestPlugin) Name() string {
	return p.customName
}

func (p *namedTestPlugin) Description() string {
	return "Named test plugin"
}

func (p *namedTestPlugin) Install(ctx *install.Context) error {
	return nil
}

func (p *namedTestPlugin) Uninstall(ctx *install.Context) error {
	return nil
}

func (p *namedTestPlugin) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (p *namedTestPlugin) DefaultConfig() interface{} {
	return map[string]interface{}{}
}

func (p *namedTestPlugin) ValidateConfig(config interface{}) error {
	return nil
}

func (p *namedTestPlugin) Metadata() Metadata {
	return Metadata{
		Name:         p.Name(),
		Description:  p.Description(),
		Dependencies: []string{},
	}
}

func TestMultiplePluginsConcurrent(t *testing.T) {
	mu.Lock()
	plugins = make(map[string]Plugin)
	mu.Unlock()

	const numPlugins = 50

	errChan := make(chan error, numPlugins)

	for i := 0; i < numPlugins; i++ {
		go func(id int) {
			plugin := &namedTestPlugin{
				customName: fmt.Sprintf("test-%d", id),
			}
			errChan <- Register(plugin)
		}(i)
	}

	successCount := 0
	for i := 0; i < numPlugins; i++ {
		err := <-errChan
		if err == nil {
			successCount++
		}
	}

	if successCount != numPlugins {
		t.Errorf("Expected %d successful registrations, got %d", numPlugins, successCount)
	}
}

func TestPluginListEmpty(t *testing.T) {
	mu.Lock()
	plugins = make(map[string]Plugin)
	mu.Unlock()

	list := List()
	if len(list) != 0 {
		t.Errorf("Expected empty plugin list, got %d plugins", len(list))
	}
}

func TestPluginListMultiple(t *testing.T) {
	mu.Lock()
	plugins = make(map[string]Plugin)
	mu.Unlock()

	plugin1 := &testPlugin{}
	Register(plugin1)

	list := List()
	if len(list) != 1 {
		t.Errorf("Expected 1 plugin, got %d", len(list))
	}
}
