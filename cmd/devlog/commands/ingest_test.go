package commands

import (
	"os"
	"path/filepath"
	"testing"

	"devlog/internal/ingest"
)

func TestFindGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	repoDir := filepath.Join(tmpDir, "project", "subdir")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create test directories: %v", err)
	}

	gitDir := filepath.Join(tmpDir, "project", ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	t.Run("finds repo from root", func(t *testing.T) {
		result, err := ingest.FindGitRepo(filepath.Join(tmpDir, "project"))
		if err != nil {
			t.Fatalf("Expected to find git repo, got error: %v", err)
		}

		expected := "project"
		if result != expected {
			t.Errorf("Expected %s, got %s", expected, result)
		}
	})

	t.Run("finds repo from subdirectory", func(t *testing.T) {
		result, err := ingest.FindGitRepo(repoDir)
		if err != nil {
			t.Fatalf("Expected to find git repo, got error: %v", err)
		}

		expected := "project"
		if result != expected {
			t.Errorf("Expected %s, got %s", expected, result)
		}
	})

	t.Run("returns error when not in git repo", func(t *testing.T) {
		nonGitDir := filepath.Join(tmpDir, "notgit")
		if err := os.MkdirAll(nonGitDir, 0755); err != nil {
			t.Fatalf("Failed to create non-git directory: %v", err)
		}

		_, err := ingest.FindGitRepo(nonGitDir)
		if err == nil {
			t.Error("Expected error when not in git repo")
		}
	})

	t.Run("handles root directory", func(t *testing.T) {
		_, err := ingest.FindGitRepo("/")
		if err == nil {
			t.Error("Expected error when searching from root")
		}
	})
}

func TestFindGitRepoNestedRepos(t *testing.T) {
	tmpDir := t.TempDir()

	outerRepoDir := filepath.Join(tmpDir, "outer")
	innerRepoDir := filepath.Join(outerRepoDir, "inner")

	if err := os.MkdirAll(innerRepoDir, 0755); err != nil {
		t.Fatalf("Failed to create directories: %v", err)
	}

	outerGitDir := filepath.Join(outerRepoDir, ".git")
	if err := os.MkdirAll(outerGitDir, 0755); err != nil {
		t.Fatalf("Failed to create outer .git: %v", err)
	}

	innerGitDir := filepath.Join(innerRepoDir, ".git")
	if err := os.MkdirAll(innerGitDir, 0755); err != nil {
		t.Fatalf("Failed to create inner .git: %v", err)
	}

	t.Run("finds nearest repo", func(t *testing.T) {
		result, err := ingest.FindGitRepo(innerRepoDir)
		if err != nil {
			t.Fatalf("Expected to find git repo, got error: %v", err)
		}

		expected := "inner"
		if result != expected {
			t.Errorf("Expected %s, got %s (should find nearest repo)", expected, result)
		}
	})
}
