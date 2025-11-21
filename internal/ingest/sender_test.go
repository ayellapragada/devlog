package ingest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindGitRepo(t *testing.T) {
	testCases := []struct {
		name        string
		setup       func(t *testing.T) string
		expectError bool
		checkResult func(t *testing.T, result, tempDir string)
	}{
		{
			name: "finds git repo in current directory",
			setup: func(t *testing.T) string {
				tempDir := t.TempDir()
				gitDir := filepath.Join(tempDir, ".git")
				if err := os.Mkdir(gitDir, 0755); err != nil {
					t.Fatalf("failed to create .git dir: %v", err)
				}
				return tempDir
			},
			expectError: false,
			checkResult: func(t *testing.T, result, tempDir string) {
				if result != tempDir {
					t.Errorf("expected %q, got %q", tempDir, result)
				}
			},
		},
		{
			name: "finds git repo in parent directory",
			setup: func(t *testing.T) string {
				tempDir := t.TempDir()
				gitDir := filepath.Join(tempDir, ".git")
				if err := os.Mkdir(gitDir, 0755); err != nil {
					t.Fatalf("failed to create .git dir: %v", err)
				}
				subDir := filepath.Join(tempDir, "subdir", "nested")
				if err := os.MkdirAll(subDir, 0755); err != nil {
					t.Fatalf("failed to create nested dir: %v", err)
				}
				return subDir
			},
			expectError: false,
			checkResult: func(t *testing.T, result, tempDir string) {
				expected := filepath.Dir(filepath.Dir(tempDir))
				if result != expected {
					t.Errorf("expected %q, got %q", expected, result)
				}
			},
		},
		{
			name: "returns error when not in git repo",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			expectError: true,
			checkResult: func(t *testing.T, result, tempDir string) {},
		},
		{
			name: "finds closest git repo with nested repos",
			setup: func(t *testing.T) string {
				tempDir := t.TempDir()
				outerGit := filepath.Join(tempDir, ".git")
				if err := os.Mkdir(outerGit, 0755); err != nil {
					t.Fatalf("failed to create outer .git: %v", err)
				}

				innerDir := filepath.Join(tempDir, "inner")
				if err := os.Mkdir(innerDir, 0755); err != nil {
					t.Fatalf("failed to create inner dir: %v", err)
				}
				innerGit := filepath.Join(innerDir, ".git")
				if err := os.Mkdir(innerGit, 0755); err != nil {
					t.Fatalf("failed to create inner .git: %v", err)
				}

				return innerDir
			},
			expectError: false,
			checkResult: func(t *testing.T, result, tempDir string) {
				if result != tempDir {
					t.Errorf("expected %q (inner repo), got %q", tempDir, result)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			startPath := tc.setup(t)

			result, err := FindGitRepo(startPath)

			if tc.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if result != "" {
					t.Errorf("expected empty result on error, got %q", result)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				tc.checkResult(t, result, startPath)
			}
		})
	}
}
