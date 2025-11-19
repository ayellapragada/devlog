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
}

func NewManager(dataDir string) (*Manager, error) {
	filePath := filepath.Join(dataDir, "poller_state.json")
	m := &Manager{
		filePath: filePath,
	}

	return m, nil
}

func (m *Manager) readState() (map[string]map[string]interface{}, error) {
	data := make(map[string]map[string]interface{})

	fileData, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(fileData, &data); err != nil {
		return nil, err
	}

	return data, nil
}

func (m *Manager) writeState(data map[string]map[string]interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	return os.WriteFile(m.filePath, jsonData, 0644)
}

func (m *Manager) Get(module, key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := m.readState()
	if err != nil {
		return nil, false
	}

	moduleData, exists := data[module]
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

	data, err := m.readState()
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}

	if data[module] == nil {
		data[module] = make(map[string]interface{})
	}

	data[module][key] = value

	return m.writeState(data)
}

func (m *Manager) Delete(module, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := m.readState()
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}

	if moduleData, exists := data[module]; exists {
		delete(moduleData, key)
		if len(moduleData) == 0 {
			delete(data, module)
		}
	}

	return m.writeState(data)
}

func (m *Manager) DeleteModule(module string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := m.readState()
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}

	delete(data, module)

	return m.writeState(data)
}
