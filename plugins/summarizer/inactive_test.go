package summarizer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"devlog/internal/logger"
)

func TestUpdateOrCreateInactivePeriod(t *testing.T) {
	tmpDir := t.TempDir()
	summaryFile := filepath.Join(tmpDir, "summary_2025-11-21.md")

	p := &Plugin{
		logger: logger.Default(),
	}

	t1, _ := time.Parse("15:04", "00:00")
	t2, _ := time.Parse("15:04", "00:30")
	t3, _ := time.Parse("15:04", "01:00")
	t4, _ := time.Parse("15:04", "01:30")

	if err := p.updateOrCreateInactivePeriod(summaryFile, t1, t2); err != nil {
		t.Fatalf("first inactive period failed: %v", err)
	}

	content, _ := os.ReadFile(summaryFile)
	if !strings.Contains(string(content), "## 00:00 - 00:30") {
		t.Errorf("expected first period, got:\n%s", content)
	}

	if err := p.updateOrCreateInactivePeriod(summaryFile, t2, t3); err != nil {
		t.Fatalf("second inactive period failed: %v", err)
	}

	content, _ = os.ReadFile(summaryFile)
	if !strings.Contains(string(content), "## 00:00 - 01:00") {
		t.Errorf("expected consolidated period, got:\n%s", content)
	}
	if strings.Contains(string(content), "00:30") {
		t.Errorf("should not contain 00:30, got:\n%s", content)
	}

	if err := p.updateOrCreateInactivePeriod(summaryFile, t3, t4); err != nil {
		t.Fatalf("third inactive period failed: %v", err)
	}

	content, _ = os.ReadFile(summaryFile)
	if !strings.Contains(string(content), "## 00:00 - 01:30") {
		t.Errorf("expected triple-consolidated period, got:\n%s", content)
	}

	lines := strings.Count(string(content), "No development activity")
	if lines != 1 {
		t.Errorf("expected exactly 1 inactive message, got %d:\n%s", lines, content)
	}
}

func TestUpdateOrCreateInactivePeriod_NonConsecutive(t *testing.T) {
	tmpDir := t.TempDir()
	summaryFile := filepath.Join(tmpDir, "summary_2025-11-21.md")

	p := &Plugin{
		logger: logger.Default(),
	}

	t1, _ := time.Parse("15:04", "00:00")
	t2, _ := time.Parse("15:04", "00:30")
	t3, _ := time.Parse("15:04", "02:00")
	t4, _ := time.Parse("15:04", "02:30")

	if err := p.updateOrCreateInactivePeriod(summaryFile, t1, t2); err != nil {
		t.Fatalf("first inactive period failed: %v", err)
	}

	if err := p.updateOrCreateInactivePeriod(summaryFile, t3, t4); err != nil {
		t.Fatalf("second inactive period failed: %v", err)
	}

	content, _ := os.ReadFile(summaryFile)
	if !strings.Contains(string(content), "## 00:00 - 00:30") {
		t.Errorf("expected first period to remain, got:\n%s", content)
	}
	if !strings.Contains(string(content), "## 02:00 - 02:30") {
		t.Errorf("expected second period to be separate, got:\n%s", content)
	}

	lines := strings.Count(string(content), "No development activity")
	if lines != 2 {
		t.Errorf("expected exactly 2 inactive messages, got %d:\n%s", lines, content)
	}
}
