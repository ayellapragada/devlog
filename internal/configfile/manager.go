package configfile

import (
	"fmt"
	"os"
	"strings"
)

type Manager interface {
	AddSection(path, marker, content string) error
	RemoveSection(path, marker string) error
	HasSection(path, marker string) (bool, error)
}

type FileSystemManager struct {
	backupSuffix string
}

func NewFileSystemManager(backupSuffix string) *FileSystemManager {
	if backupSuffix == "" {
		backupSuffix = ".backup.devlog"
	}
	return &FileSystemManager{
		backupSuffix: backupSuffix,
	}
}

func (m *FileSystemManager) AddSection(path, marker, content string) error {
	var existingContent []byte
	var fileExists bool

	if _, err := os.Stat(path); err == nil {
		fileExists = true
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read file %s: %w", path, err)
		}
		existingContent = data

		if m.containsMarker(string(existingContent), marker) {
			return fmt.Errorf("section '%s' already exists in %s", marker, path)
		}

		backupPath := path + m.backupSuffix
		if err := os.WriteFile(backupPath, existingContent, 0644); err != nil {
			return fmt.Errorf("create backup at %s: %w", backupPath, err)
		}
	}

	var newContent string
	if fileExists {
		contentStr := string(existingContent)
		if !strings.HasSuffix(contentStr, "\n") {
			contentStr += "\n"
		}
		newContent = contentStr + "\n# " + marker + "\n" + content + "\n"
	} else {
		newContent = "# " + marker + "\n" + content + "\n"
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

func (m *FileSystemManager) RemoveSection(path, marker string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read file %s: %w", path, err)
	}

	contentStr := string(data)
	if !m.containsMarker(contentStr, marker) {
		return nil
	}

	backupPath := path + m.backupSuffix + ".uninstall"
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("create backup at %s: %w", backupPath, err)
	}

	lines := strings.Split(contentStr, "\n")
	var newLines []string
	inSection := false

	for _, line := range lines {
		if strings.Contains(line, "# "+marker) {
			inSection = true
			continue
		}

		if inSection {
			if strings.TrimSpace(line) == "" {
				inSection = false
				continue
			}
			if !strings.HasPrefix(strings.TrimSpace(line), "#") && strings.TrimSpace(line) != "" {
				inSection = false
			}
			if inSection {
				continue
			}
		}

		newLines = append(newLines, line)
	}

	for len(newLines) > 0 && strings.TrimSpace(newLines[len(newLines)-1]) == "" {
		newLines = newLines[:len(newLines)-1]
	}

	newContent := strings.Join(newLines, "\n")
	if len(newLines) > 0 {
		newContent += "\n"
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

func (m *FileSystemManager) HasSection(path, marker string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read file %s: %w", path, err)
	}

	return m.containsMarker(string(data), marker), nil
}

func (m *FileSystemManager) containsMarker(content, marker string) bool {
	return strings.Contains(content, "# "+marker) || strings.Contains(content, marker)
}
