package metrics

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestSnapshot_RecordPluginStart(t *testing.T) {
	s := NewSnapshot()
	pluginName := "test_plugin"

	s.RecordPluginStart(pluginName)

	if _, exists := s.PluginStartTime[pluginName]; !exists {
		t.Errorf("plugin start time not recorded")
	}
}

func TestSnapshot_RecordPluginError(t *testing.T) {
	s := NewSnapshot()
	pluginName := "test_plugin"
	testErr := errors.New("test error")

	s.RecordPluginError(pluginName, testErr)

	if s.PluginErrorCount[pluginName] != 1 {
		t.Errorf("error count = %d, want 1", s.PluginErrorCount[pluginName])
	}
	if s.PluginLastError[pluginName] != testErr.Error() {
		t.Errorf("last error = %q, want %q", s.PluginLastError[pluginName], testErr.Error())
	}

	s.RecordPluginError(pluginName, testErr)
	if s.PluginErrorCount[pluginName] != 2 {
		t.Errorf("error count after second error = %d, want 2", s.PluginErrorCount[pluginName])
	}
}

func TestSnapshot_RecordPluginRestart(t *testing.T) {
	s := NewSnapshot()
	pluginName := "test_plugin"

	s.RecordPluginRestart(pluginName)
	s.RecordPluginRestart(pluginName)

	if s.PluginRestarts[pluginName] != 2 {
		t.Errorf("restart count = %d, want 2", s.PluginRestarts[pluginName])
	}
}

func TestSnapshot_RecordEventIngested(t *testing.T) {
	s := NewSnapshot()

	s.RecordEventIngested("git", "commit")
	s.RecordEventIngested("git", "push")
	s.RecordEventIngested("shell", "command")

	if s.EventsIngested != 3 {
		t.Errorf("events ingested = %d, want 3", s.EventsIngested)
	}
	if s.EventsBySource["git"] != 2 {
		t.Errorf("git events = %d, want 2", s.EventsBySource["git"])
	}
	if s.EventsByType["commit"] != 1 {
		t.Errorf("commit events = %d, want 1", s.EventsByType["commit"])
	}
}

func TestSnapshot_RecordEventIngested_Concurrent(t *testing.T) {
	s := NewSnapshot()
	goroutines := 100
	eventsPerGoroutine := 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			source := "git"
			if id%2 == 0 {
				source = "shell"
			}
			for j := 0; j < eventsPerGoroutine; j++ {
				s.RecordEventIngested(source, "commit")
			}
		}(i)
	}

	wg.Wait()

	expected := int64(goroutines * eventsPerGoroutine)
	if s.EventsIngested != expected {
		t.Errorf("events ingested = %d, want %d", s.EventsIngested, expected)
	}

	gitEvents := s.EventsBySource["git"]
	shellEvents := s.EventsBySource["shell"]
	total := gitEvents + shellEvents

	if total != expected {
		t.Errorf("total events by source = %d, want %d", total, expected)
	}
}

func TestSnapshot_TimeBuckets(t *testing.T) {
	s := NewSnapshot()

	s.RecordEventIngested("git", "commit")

	if len(s.HourlyBuckets) == 0 {
		t.Error("hourly buckets not created")
	}
	if len(s.DailyBuckets) == 0 {
		t.Error("daily buckets not created")
	}

	now := time.Now()
	hourKey := now.Unix() / 3600

	bucket, exists := s.HourlyBuckets[hourKey]
	if !exists {
		t.Fatal("current hour bucket not found")
	}
	if bucket.TotalEvents != 1 {
		t.Errorf("hourly bucket total = %d, want 1", bucket.TotalEvents)
	}
	if bucket.EventsBySource["git"] != 1 {
		t.Errorf("hourly bucket git events = %d, want 1", bucket.EventsBySource["git"])
	}
}

func TestSnapshot_MaxSourceTypesLimit(t *testing.T) {
	s := NewSnapshot()

	for i := 0; i < MaxSourceTypes+10; i++ {
		source := string(rune('a' + i))
		s.RecordEventIngested(source, "commit")
	}

	if len(s.EventsBySource) > MaxSourceTypes {
		t.Errorf("source types = %d, want <= %d", len(s.EventsBySource), MaxSourceTypes)
	}
}

func TestSnapshot_MaxEventTypesLimit(t *testing.T) {
	s := NewSnapshot()

	for i := 0; i < MaxEventTypes+10; i++ {
		eventType := string(rune('a' + i))
		s.RecordEventIngested("git", eventType)
	}

	if len(s.EventsByType) > MaxEventTypes {
		t.Errorf("event types = %d, want <= %d", len(s.EventsByType), MaxEventTypes)
	}
}

func TestSnapshot_UpdateSystemMetrics(t *testing.T) {
	s := NewSnapshot()
	time.Sleep(1100 * time.Millisecond)

	queueDepth := int64(42)
	dbSize := int64(1024 * 1024)
	eventCount := int64(1000)

	s.UpdateSystemMetrics(queueDepth, dbSize, eventCount)

	if s.QueueDepth != queueDepth {
		t.Errorf("queue depth = %d, want %d", s.QueueDepth, queueDepth)
	}
	if s.DatabaseSize != dbSize {
		t.Errorf("database size = %d, want %d", s.DatabaseSize, dbSize)
	}
	if s.EventCount != eventCount {
		t.Errorf("event count = %d, want %d", s.EventCount, eventCount)
	}
	if s.UptimeSeconds <= 0 {
		t.Errorf("uptime = %d, want > 0", s.UptimeSeconds)
	}
}

func TestSnapshot_Copy(t *testing.T) {
	s := NewSnapshot()
	s.RecordEventIngested("git", "commit")
	s.RecordPluginStart("test_plugin")
	s.RecordPluginError("test_plugin", errors.New("test error"))

	copy := s.Copy()

	if copy.EventsIngested != s.EventsIngested {
		t.Error("events ingested not copied")
	}
	if len(copy.EventsBySource) != len(s.EventsBySource) {
		t.Error("events by source not copied")
	}
	if len(copy.PluginStartTime) != len(s.PluginStartTime) {
		t.Error("plugin start times not copied")
	}

	copy.EventsIngested = 999
	if s.EventsIngested == 999 {
		t.Error("copy is not independent from original")
	}
}

func TestSnapshot_WriteToFile(t *testing.T) {
	s := NewSnapshot()
	s.RecordEventIngested("git", "commit")
	s.RecordPluginStart("test_plugin")

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "snapshot.json")

	if err := s.WriteToFile(path); err != nil {
		t.Fatalf("WriteToFile failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("snapshot file is empty")
	}
}

func TestSnapshot_ToJSON(t *testing.T) {
	s := NewSnapshot()
	s.RecordEventIngested("git", "commit")

	data, err := s.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("JSON output is empty")
	}
}

func TestSnapshot_GetSummary(t *testing.T) {
	s := NewSnapshot()
	s.RecordEventIngested("git", "commit")
	s.RecordPluginStart("test_plugin")
	s.UpdateSystemMetrics(10, 1024, 100)

	summary := s.GetSummary()

	if summary.EventCount != 100 {
		t.Errorf("summary event count = %d, want 100", summary.EventCount)
	}
	if summary.QueueDepth != 10 {
		t.Errorf("summary queue depth = %d, want 10", summary.QueueDepth)
	}
	if len(summary.EventsBySource) == 0 {
		t.Error("summary events by source is empty")
	}
	if len(summary.PluginStatus) == 0 {
		t.Error("summary plugin status is empty")
	}
}

func TestSnapshot_GetSummary_PluginStatus(t *testing.T) {
	s := NewSnapshot()
	s.RecordPluginStart("healthy_plugin")
	s.RecordPluginStart("error_plugin")
	s.RecordPluginError("error_plugin", errors.New("test error"))

	time.Sleep(6 * time.Second)

	summary := s.GetSummary()

	if summary.PluginStatus["healthy_plugin"] != "running" {
		t.Errorf("healthy plugin status = %q, want %q", summary.PluginStatus["healthy_plugin"], "running")
	}
	if summary.PluginStatus["error_plugin"] != "running (with errors)" {
		t.Errorf("error plugin status = %q, want %q", summary.PluginStatus["error_plugin"], "running (with errors)")
	}
	if summary.ErrorCount != 1 {
		t.Errorf("error count = %d, want 1", summary.ErrorCount)
	}
}

func TestSnapshot_CleanupOldBuckets(t *testing.T) {
	s := NewSnapshot()

	now := time.Now()
	oldHourKey := (now.Unix() / 3600) - HourlyBucketsTTL - 10
	oldDayKey := (now.Unix() / 86400) - DailyBucketsTTL - 10

	s.HourlyBuckets[oldHourKey] = &TimeBucket{
		EventsBySource: map[string]int64{"git": 1},
		EventsByType:   map[string]int64{"commit": 1},
		TotalEvents:    1,
	}
	s.DailyBuckets[oldDayKey] = &TimeBucket{
		EventsBySource: map[string]int64{"git": 1},
		EventsByType:   map[string]int64{"commit": 1},
		TotalEvents:    1,
	}

	s.cleanupOldBuckets()

	if _, exists := s.HourlyBuckets[oldHourKey]; exists {
		t.Error("old hourly bucket not cleaned up")
	}
	if _, exists := s.DailyBuckets[oldDayKey]; exists {
		t.Error("old daily bucket not cleaned up")
	}
}

func TestSnapshot_ConcurrentReadWrite(t *testing.T) {
	s := NewSnapshot()

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			s.RecordEventIngested("git", "commit")
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			s.RecordPluginError("test_plugin", errors.New("test"))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = s.Copy()
			_ = s.GetSummary()
		}
	}()

	wg.Wait()

	if s.EventsIngested != 1000 {
		t.Errorf("events ingested = %d, want 1000", s.EventsIngested)
	}
}

func BenchmarkSnapshot_RecordEventIngested(b *testing.B) {
	s := NewSnapshot()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		s.RecordEventIngested("git", "commit")
	}
}

func BenchmarkSnapshot_RecordEventIngested_Parallel(b *testing.B) {
	s := NewSnapshot()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s.RecordEventIngested("git", "commit")
		}
	})
}

func BenchmarkSnapshot_Copy(b *testing.B) {
	s := NewSnapshot()
	for i := 0; i < 100; i++ {
		s.RecordEventIngested("git", "commit")
		s.RecordPluginStart("plugin_" + string(rune(i)))
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = s.Copy()
	}
}
