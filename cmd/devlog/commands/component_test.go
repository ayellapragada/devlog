package commands

import (
	"testing"

	"devlog/internal/install"
)

// MockComponent implements Component interface for testing
type MockComponent struct {
	name        string
	description string
	defaultCfg  interface{}
	installErr  error
	validateErr error
}

func (m *MockComponent) Name() string {
	return m.name
}

func (m *MockComponent) Description() string {
	return m.description
}

func (m *MockComponent) Install(ctx *install.Context) error {
	return m.installErr
}

func (m *MockComponent) Uninstall(ctx *install.Context) error {
	return nil
}

func (m *MockComponent) DefaultConfig() interface{} {
	return m.defaultCfg
}

func (m *MockComponent) ValidateConfig(config interface{}) error {
	return m.validateErr
}

// MockComponentRegistry implements ComponentRegistry for testing
type MockComponentRegistry struct {
	components map[string]Component
}

func NewMockComponentRegistry() *MockComponentRegistry {
	return &MockComponentRegistry{
		components: make(map[string]Component),
	}
}

func (r *MockComponentRegistry) Get(name string) (Component, error) {
	if comp, exists := r.components[name]; exists {
		return comp, nil
	}
	return nil, ErrComponentNotFound
}

func (r *MockComponentRegistry) List() []Component {
	comps := make([]Component, 0, len(r.components))
	for _, comp := range r.components {
		comps = append(comps, comp)
	}
	return comps
}

func (r *MockComponentRegistry) Add(comp Component) {
	r.components[comp.Name()] = comp
}

// Error types for testing
type TestError string

func (e TestError) Error() string {
	return string(e)
}

const ErrComponentNotFound TestError = "component not found"

// MockComponentConfig implements ComponentConfig for testing
type MockComponentConfig struct {
	enabled map[string]bool
	config  map[string]map[string]interface{}
}

func NewMockComponentConfig() *MockComponentConfig {
	return &MockComponentConfig{
		enabled: make(map[string]bool),
		config:  make(map[string]map[string]interface{}),
	}
}

func (c *MockComponentConfig) IsEnabled(name string) bool {
	return c.enabled[name]
}

func (c *MockComponentConfig) GetConfig(name string) (map[string]interface{}, bool) {
	cfg, exists := c.config[name]
	return cfg, exists
}

func (c *MockComponentConfig) SetEnabled(name string, enabled bool) {
	c.enabled[name] = enabled
}

func (c *MockComponentConfig) SetConfig(name string, config map[string]interface{}) {
	c.config[name] = config
}

func (c *MockComponentConfig) ClearConfig(name string) {
	delete(c.config, name)
	delete(c.enabled, name)
}

func (c *MockComponentConfig) Save() error {
	return nil
}

// Tests for component helper functions
func TestCapitalize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"module", "Module"},
		{"plugin", "Plugin"},
		{"a", "A"},
		{"", ""},
		{"abc", "Abc"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := capitalize(tt.input)
			if result != tt.expected {
				t.Errorf("capitalize(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestComponentList(t *testing.T) {
	t.Run("list with components", func(t *testing.T) {
		registry := NewMockComponentRegistry()
		registry.Add(&MockComponent{
			name:        "test1",
			description: "Test component 1",
		})
		registry.Add(&MockComponent{
			name:        "test2",
			description: "Test component 2",
		})

		components := registry.List()
		if len(components) != 2 {
			t.Errorf("expected 2 components, got %d", len(components))
		}
	})

	t.Run("list is empty", func(t *testing.T) {
		registry := NewMockComponentRegistry()
		components := registry.List()
		if len(components) != 0 {
			t.Errorf("expected 0 components, got %d", len(components))
		}
	})
}

func TestComponentInstall(t *testing.T) {
	t.Run("successful install", func(t *testing.T) {
		registry := NewMockComponentRegistry()
		config := NewMockComponentConfig()

		comp := &MockComponent{
			name:        "test",
			description: "Test component",
			defaultCfg: map[string]interface{}{
				"key": "value",
			},
		}
		registry.Add(comp)

		// Component should exist in registry
		retrieved, err := registry.Get("test")
		if err != nil {
			t.Errorf("get component: %v", err)
		}
		if retrieved.Name() != "test" {
			t.Errorf("wrong component: %s", retrieved.Name())
		}

		// Config should be settable
		config.SetEnabled("test", true)
		if !config.IsEnabled("test") {
			t.Error("component should be enabled")
		}
	})

	t.Run("component not found", func(t *testing.T) {
		registry := NewMockComponentRegistry()

		_, err := registry.Get("nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent component")
		}
	})

	t.Run("component with config", func(t *testing.T) {
		config := NewMockComponentConfig()

		cfgData := map[string]interface{}{
			"enabled": true,
			"timeout": 30,
		}
		config.SetConfig("test", cfgData)

		retrieved, exists := config.GetConfig("test")
		if !exists {
			t.Error("config should exist")
		}
		if retrieved["timeout"] != 30 {
			t.Errorf("expected timeout=30, got %v", retrieved["timeout"])
		}
	})
}

func TestComponentUninstall(t *testing.T) {
	t.Run("uninstall removes config", func(t *testing.T) {
		config := NewMockComponentConfig()

		// Set enabled and config
		config.SetEnabled("test", true)
		config.SetConfig("test", map[string]interface{}{"key": "value"})

		if !config.IsEnabled("test") {
			t.Error("component should be enabled")
		}

		// Clear config
		config.ClearConfig("test")

		if config.IsEnabled("test") {
			t.Error("component should not be enabled after clear")
		}

		_, exists := config.GetConfig("test")
		if exists {
			t.Error("config should not exist after clear")
		}
	})

	t.Run("uninstall without purge", func(t *testing.T) {
		config := NewMockComponentConfig()

		config.SetEnabled("test", true)
		config.SetConfig("test", map[string]interface{}{"key": "value"})

		// Disable without clearing
		config.SetEnabled("test", false)

		if config.IsEnabled("test") {
			t.Error("component should be disabled")
		}

		// Config should still exist
		_, exists := config.GetConfig("test")
		if !exists {
			t.Error("config should still exist when not purged")
		}
	})
}

func TestComponentRegistry(t *testing.T) {
	t.Run("registry operations", func(t *testing.T) {
		registry := NewMockComponentRegistry()

		comp1 := &MockComponent{name: "comp1", description: "Component 1"}
		comp2 := &MockComponent{name: "comp2", description: "Component 2"}

		registry.Add(comp1)
		registry.Add(comp2)

		// Get individual components
		retrieved, err := registry.Get("comp1")
		if err != nil {
			t.Errorf("get comp1: %v", err)
		}
		if retrieved.Name() != "comp1" {
			t.Errorf("wrong component: %s", retrieved.Name())
		}

		// List all components
		all := registry.List()
		if len(all) != 2 {
			t.Errorf("expected 2 components, got %d", len(all))
		}
	})

	t.Run("registry handles duplicates", func(t *testing.T) {
		registry := NewMockComponentRegistry()

		comp1 := &MockComponent{name: "test", description: "First"}
		comp2 := &MockComponent{name: "test", description: "Second"}

		registry.Add(comp1)
		registry.Add(comp2)

		// Should have overwritten
		retrieved, _ := registry.Get("test")
		if retrieved.Description() != "Second" {
			t.Errorf("expected 'Second', got %q", retrieved.Description())
		}
	})
}

func TestComponentConfig(t *testing.T) {
	t.Run("config operations", func(t *testing.T) {
		config := NewMockComponentConfig()

		// Set enabled
		config.SetEnabled("module1", true)
		if !config.IsEnabled("module1") {
			t.Error("module1 should be enabled")
		}

		// Set config
		cfg := map[string]interface{}{
			"option1": "value1",
			"option2": 42,
		}
		config.SetConfig("module1", cfg)

		retrieved, exists := config.GetConfig("module1")
		if !exists {
			t.Error("config should exist")
		}
		if retrieved["option1"] != "value1" {
			t.Errorf("option1 mismatch")
		}
		if retrieved["option2"] != 42 {
			t.Errorf("option2 mismatch")
		}
	})

	t.Run("config clear", func(t *testing.T) {
		config := NewMockComponentConfig()

		config.SetEnabled("test", true)
		config.SetConfig("test", map[string]interface{}{"key": "value"})

		config.ClearConfig("test")

		if config.IsEnabled("test") {
			t.Error("should not be enabled after clear")
		}

		_, exists := config.GetConfig("test")
		if exists {
			t.Error("config should not exist after clear")
		}
	})

	t.Run("config save", func(t *testing.T) {
		config := NewMockComponentConfig()

		config.SetEnabled("test", true)
		config.SetConfig("test", map[string]interface{}{"key": "value"})

		err := config.Save()
		if err != nil {
			t.Errorf("save: %v", err)
		}
	})
}
