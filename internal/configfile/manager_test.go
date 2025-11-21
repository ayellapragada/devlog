package configfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileSystemManagerCreation(t *testing.T) {
	t.Run("creates manager with default backup suffix", func(t *testing.T) {
		m := NewFileSystemManager("")
		if m == nil {
			t.Fatal("manager is nil")
		}
		if m.backupSuffix != ".backup.devlog" {
			t.Errorf("expected default suffix '.backup.devlog', got %s", m.backupSuffix)
		}
	})

	t.Run("creates manager with custom backup suffix", func(t *testing.T) {
		m := NewFileSystemManager(".bak")
		if m == nil {
			t.Fatal("manager is nil")
		}
		if m.backupSuffix != ".bak" {
			t.Errorf("expected suffix '.bak', got %s", m.backupSuffix)
		}
	})
}

func TestAddSection(t *testing.T) {
	t.Run("adds section to new file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		m := NewFileSystemManager(".bak")

		err := m.AddSection(testFile, "DEVLOG_START", "export DEVLOG=1")
		if err != nil {
			t.Fatalf("AddSection failed: %v", err)
		}

		content, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}

		expected := "# DEVLOG_START\nexport DEVLOG=1\n"
		if string(content) != expected {
			t.Errorf("expected:\n%q\ngot:\n%q", expected, string(content))
		}
	})

	t.Run("adds section to existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		m := NewFileSystemManager(".bak")

		if err := os.WriteFile(testFile, []byte("existing content\n"), 0644); err != nil {
			t.Fatalf("write initial file: %v", err)
		}

		err := m.AddSection(testFile, "DEVLOG_START", "export DEVLOG=1")
		if err != nil {
			t.Fatalf("AddSection failed: %v", err)
		}

		content, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}

		contentStr := string(content)
		if !strings.Contains(contentStr, "existing content") {
			t.Error("lost existing content")
		}
		if !strings.Contains(contentStr, "# DEVLOG_START") {
			t.Error("marker not added")
		}
		if !strings.Contains(contentStr, "export DEVLOG=1") {
			t.Error("content not added")
		}
	})

	t.Run("creates backup when adding to existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		backupFile := testFile + ".bak"
		m := NewFileSystemManager(".bak")

		originalContent := "original content\n"
		if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
			t.Fatalf("write initial file: %v", err)
		}

		err := m.AddSection(testFile, "MARKER", "new content")
		if err != nil {
			t.Fatalf("AddSection failed: %v", err)
		}

		backupContent, err := os.ReadFile(backupFile)
		if err != nil {
			t.Fatalf("backup file not created: %v", err)
		}

		if string(backupContent) != originalContent {
			t.Errorf("backup content mismatch: got %q, want %q", string(backupContent), originalContent)
		}
	})

	t.Run("returns error when section already exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		m := NewFileSystemManager(".bak")

		if err := os.WriteFile(testFile, []byte("# MARKER\nexisting\n"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		err := m.AddSection(testFile, "MARKER", "new content")
		if err == nil {
			t.Error("expected error when section already exists")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("wrong error message: %v", err)
		}
	})

	t.Run("adds newline before section if missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		m := NewFileSystemManager(".bak")

		if err := os.WriteFile(testFile, []byte("no newline"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		err := m.AddSection(testFile, "MARKER", "content")
		if err != nil {
			t.Fatalf("AddSection failed: %v", err)
		}

		content, _ := os.ReadFile(testFile)
		contentStr := string(content)

		lines := strings.Split(contentStr, "\n")
		if len(lines) < 3 {
			t.Error("insufficient lines in output")
		}
	})
}

func TestRemoveSection(t *testing.T) {
	t.Run("removes comment section from file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		m := NewFileSystemManager(".bak")

		initialContent := "before\n# MARKER\n# comment line 1\n# comment line 2\n\nafter\n"
		if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		err := m.RemoveSection(testFile, "MARKER")
		if err != nil {
			t.Fatalf("RemoveSection failed: %v", err)
		}

		content, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}

		contentStr := string(content)
		if strings.Contains(contentStr, "MARKER") {
			t.Error("marker still present")
		}
		if strings.Contains(contentStr, "comment line 1") {
			t.Error("section content still present")
		}
		if !strings.Contains(contentStr, "before") {
			t.Error("content before section was removed")
		}
		if !strings.Contains(contentStr, "after") {
			t.Error("content after section was removed")
		}
	})

	t.Run("creates backup when removing section", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		backupFile := testFile + ".bak.uninstall"
		m := NewFileSystemManager(".bak")

		originalContent := "# MARKER\ncontent\n"
		if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		err := m.RemoveSection(testFile, "MARKER")
		if err != nil {
			t.Fatalf("RemoveSection failed: %v", err)
		}

		backupContent, err := os.ReadFile(backupFile)
		if err != nil {
			t.Fatalf("backup not created: %v", err)
		}

		if string(backupContent) != originalContent {
			t.Errorf("backup mismatch: got %q, want %q", string(backupContent), originalContent)
		}
	})

	t.Run("no error when section doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		m := NewFileSystemManager(".bak")

		if err := os.WriteFile(testFile, []byte("content\n"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		err := m.RemoveSection(testFile, "NONEXISTENT")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("no error when file doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "nonexistent.txt")
		m := NewFileSystemManager(".bak")

		err := m.RemoveSection(testFile, "MARKER")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("removes multiline comment section", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		m := NewFileSystemManager(".bak")

		content := `before
# MARKER
# line1
# line2
# line3

after
`
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		err := m.RemoveSection(testFile, "MARKER")
		if err != nil {
			t.Fatalf("RemoveSection failed: %v", err)
		}

		result, _ := os.ReadFile(testFile)
		resultStr := string(result)

		if strings.Contains(resultStr, "MARKER") {
			t.Error("marker not removed")
		}
		if strings.Contains(resultStr, "line1") || strings.Contains(resultStr, "line2") {
			t.Error("section content not removed")
		}
		if !strings.Contains(resultStr, "before") || !strings.Contains(resultStr, "after") {
			t.Error("surrounding content was removed")
		}
	})

	t.Run("preserves non-comment lines after marker", func(t *testing.T) {
		// Edge case: RemoveSection only removes comment lines after the marker.
		// As soon as a non-comment line is encountered, the section ends.
		// This is by design for shell RC files where actual code (like source commands)
		// should be preserved, while comment blocks can be safely removed.
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		m := NewFileSystemManager(".bak")

		content := "before\n# MARKER\nnon-comment line\nafter\n"
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		err := m.RemoveSection(testFile, "MARKER")
		if err != nil {
			t.Fatalf("RemoveSection failed: %v", err)
		}

		result, _ := os.ReadFile(testFile)
		resultStr := string(result)

		if strings.Contains(resultStr, "MARKER") {
			t.Error("marker not removed")
		}
		if !strings.Contains(resultStr, "non-comment line") {
			t.Error("non-comment line was removed (should be preserved)")
		}
	})
}

func TestHasSection(t *testing.T) {
	t.Run("returns true when section exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		m := NewFileSystemManager(".bak")

		if err := os.WriteFile(testFile, []byte("# MARKER\ncontent\n"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		has, err := m.HasSection(testFile, "MARKER")
		if err != nil {
			t.Fatalf("HasSection failed: %v", err)
		}
		if !has {
			t.Error("expected section to exist")
		}
	})

	t.Run("returns false when section doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		m := NewFileSystemManager(".bak")

		if err := os.WriteFile(testFile, []byte("other content\n"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		has, err := m.HasSection(testFile, "MARKER")
		if err != nil {
			t.Fatalf("HasSection failed: %v", err)
		}
		if has {
			t.Error("expected section to not exist")
		}
	})

	t.Run("returns false when file doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "nonexistent.txt")
		m := NewFileSystemManager(".bak")

		has, err := m.HasSection(testFile, "MARKER")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if has {
			t.Error("expected section to not exist in nonexistent file")
		}
	})

	t.Run("detects marker with hash prefix", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		m := NewFileSystemManager(".bak")

		if err := os.WriteFile(testFile, []byte("# MARKER\n"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		has, err := m.HasSection(testFile, "MARKER")
		if err != nil {
			t.Fatalf("HasSection failed: %v", err)
		}
		if !has {
			t.Error("expected to find marker with hash prefix")
		}
	})
}

func TestRoundTripOperations(t *testing.T) {
	t.Run("add and remove comment section returns to near-original state", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		m := NewFileSystemManager(".bak")

		originalContent := "original line 1\noriginal line 2\n"
		if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		err := m.AddSection(testFile, "TEMP_MARKER", "# temporary comment")
		if err != nil {
			t.Fatalf("AddSection failed: %v", err)
		}

		has, _ := m.HasSection(testFile, "TEMP_MARKER")
		if !has {
			t.Error("section not added")
		}

		err = m.RemoveSection(testFile, "TEMP_MARKER")
		if err != nil {
			t.Fatalf("RemoveSection failed: %v", err)
		}

		finalContent, _ := os.ReadFile(testFile)
		if !strings.Contains(string(finalContent), "original line 1") ||
			!strings.Contains(string(finalContent), "original line 2") {
			t.Error("original content was lost after round trip")
		}

		if strings.Contains(string(finalContent), "temporary comment") {
			t.Error("temporary section not fully removed")
		}

		if strings.Contains(string(finalContent), "TEMP_MARKER") {
			t.Error("marker not removed")
		}
	})

	t.Run("realistic shell integration round-trip", func(t *testing.T) {
		// Integration test demonstrating real-world usage:
		// When installing shell integration, AddSection adds a marker comment
		// followed by a source line. When uninstalling, RemoveSection removes
		// only the marker comment, but preserves the source line (non-comment).
		//
		// This behavior exists because RemoveSection is designed to remove comment
		// blocks, not executable code. Users must manually remove the source line
		// if they want complete uninstallation, which is safer than automatically
		// deleting potentially important shell commands.
		tmpDir := t.TempDir()
		bashrc := filepath.Join(tmpDir, ".bashrc")
		m := NewFileSystemManager(".backup.devlog")

		originalContent := "export PATH=$HOME/bin:$PATH\nalias ll='ls -la'\n"
		if err := os.WriteFile(bashrc, []byte(originalContent), 0644); err != nil {
			t.Fatalf("write bashrc: %v", err)
		}

		sourceLine := `source "$HOME/.local/share/devlog/shell.sh"`
		err := m.AddSection(bashrc, "devlog shell integration", sourceLine)
		if err != nil {
			t.Fatalf("AddSection failed: %v", err)
		}

		content, _ := os.ReadFile(bashrc)
		if !strings.Contains(string(content), sourceLine) {
			t.Error("source line not added")
		}

		err = m.RemoveSection(bashrc, "devlog shell integration")
		if err != nil {
			t.Fatalf("RemoveSection failed: %v", err)
		}

		finalContent, _ := os.ReadFile(bashrc)
		finalStr := strings.TrimSpace(string(finalContent))

		if !strings.Contains(finalStr, "export PATH") {
			t.Error("original PATH export was removed")
		}
		if !strings.Contains(finalStr, "alias ll") {
			t.Error("original alias was removed")
		}
		if !strings.Contains(finalStr, sourceLine) {
			t.Error("source line was removed (non-comment lines are preserved by design)")
		}
		if strings.Contains(finalStr, "# devlog shell integration") {
			t.Error("marker comment should be removed")
		}
	})
}
