package commands

import (
	"context"
	"fmt"
	"testing"
	"time"

	"devlog/internal/config"
	"devlog/internal/install"
	"devlog/internal/plugins"
)

type testPlugin struct {
	name        string
	defaultCfg  interface{}
	startCalled bool
}

func (p *testPlugin) Name() string                         { return p.name }
func (p *testPlugin) Description() string                  { return "test plugin" }
func (p *testPlugin) Install(ctx *install.Context) error   { return nil }
func (p *testPlugin) Uninstall(ctx *install.Context) error { return nil }
func (p *testPlugin) Start(ctx context.Context) error {
	p.startCalled = true
	return nil
}
func (p *testPlugin) DefaultConfig() interface{} { return p.defaultCfg }
func (p *testPlugin) ValidateConfig(config interface{}) error {
	return nil
}

func TestPluginInstallStoresStructDefaultConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := config.InitConfig(); err != nil {
		t.Fatalf("InitConfig() error: %v", err)
	}

	pluginName := fmt.Sprintf("struct-plugin-%d", time.Now().UnixNano())
	type pluginConfig struct {
		Foo string `json:"foo"`
		Bar int    `json:"bar"`
	}

	testPlugin := &testPlugin{
		name: pluginName,
		defaultCfg: &pluginConfig{
			Foo: "hello",
			Bar: 42,
		},
	}

	if err := plugins.Register(testPlugin); err != nil {
		t.Fatalf("Register() error: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() before install error: %v", err)
	}

	registry := pluginRegistry{}
	configOps := pluginConfigOps{cfg: cfg}

	if err := componentInstall("plugin", []string{pluginName}, registry, configOps); err != nil {
		t.Fatalf("componentInstall() error: %v", err)
	}

	cfg, err = config.Load()
	if err != nil {
		t.Fatalf("config.Load() after install error: %v", err)
	}

	pluginCfg, ok := cfg.GetPluginConfig(pluginName)
	if !ok {
		t.Fatalf("expected config for %s", pluginName)
	}

	if pluginCfg["foo"] != "hello" {
		t.Fatalf("expected foo=hello, got %v", pluginCfg["foo"])
	}
	switch v := pluginCfg["bar"].(type) {
	case float64:
		if int(v) != 42 {
			t.Fatalf("expected bar=42, got %v", v)
		}
	case int:
		if v != 42 {
			t.Fatalf("expected bar=42, got %v", v)
		}
	default:
		t.Fatalf("expected numeric bar value, got %T", v)
	}
}
