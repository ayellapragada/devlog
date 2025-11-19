package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.HTTP.Port != 8573 {
		t.Errorf("got port %d, want 8573", cfg.HTTP.Port)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				HTTP: HTTPConfig{Port: 8573},
			},
			wantErr: false,
		},
		{
			name: "invalid port - too low",
			config: &Config{
				HTTP: HTTPConfig{Port: 0},
			},
			wantErr: true,
		},
		{
			name: "invalid port - too high",
			config: &Config{
				HTTP: HTTPConfig{Port: 99999},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	testConfig := &Config{
		HTTP: HTTPConfig{Port: 9000},
	}

	data, err := yaml.Marshal(testConfig)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	readData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(readData, cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.HTTP.Port != 9000 {
		t.Errorf("got port %d, want 9000", cfg.HTTP.Port)
	}
}

func TestConfigDirPaths(t *testing.T) {
	configDir, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	expectedConfigDir := filepath.Join(home, ".config", "devlog")
	if configDir != expectedConfigDir {
		t.Errorf("got config dir %s, want %s", configDir, expectedConfigDir)
	}

	dataDir, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}

	expectedDataDir := filepath.Join(home, ".local", "share", "devlog")
	if dataDir != expectedDataDir {
		t.Errorf("got data dir %s, want %s", dataDir, expectedDataDir)
	}
}
