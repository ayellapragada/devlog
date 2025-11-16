package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Manager struct {
	filePath string
	mu       sync.RWMutex
	data     map[string]map[string]interface{}
}

func NewManager(dataDir string) (*Manager, error) {
	filePath := filepath.Join(dataDir, "poller_state.json")
	m := &Manager{
		filePath: filePath,
		data:     make(map[string]map[string]interface{}),
	}

	if err := m.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load state: %w", err)
	}

	return m, nil
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &m.data)
}

func (m *Manager) save() error {
	data, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	return os.WriteFile(m.filePath, data, 0644)
}

func (m *Manager) Get(module, key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	moduleData, exists := m.data[module]
	if !exists {
		return nil, false
	}

	value, exists := moduleData[key]
	return value, exists
}

func (m *Manager) GetString(module, key string) (string, bool) {
	value, exists := m.Get(module, key)
	if !exists {
		return "", false
	}

	str, ok := value.(string)
	return str, ok
}

func (m *Manager) Set(module, key string, value interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.data[module] == nil {
		m.data[module] = make(map[string]interface{})
	}

	m.data[module][key] = value

	return m.save()
}

func (m *Manager) Delete(module, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if moduleData, exists := m.data[module]; exists {
		delete(moduleData, key)
		if len(moduleData) == 0 {
			delete(m.data, module)
		}
	}

	return m.save()
}

func (m *Manager) DeleteModule(module string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, module)

	return m.save()
}
