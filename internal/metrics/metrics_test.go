package metrics

import (
	"sync"
	"testing"
	"time"
)

func TestCounter_ConcurrentAccess(t *testing.T) {
	counter := &Counter{}
	goroutines := 100
	incrementsPerGoroutine := 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				counter.Add(1)
			}
		}()
	}

	wg.Wait()

	expected := int64(goroutines * incrementsPerGoroutine)
	if counter.Value() != expected {
		t.Errorf("counter value = %d, want %d", counter.Value(), expected)
	}
}

func TestTimer(t *testing.T) {
	timer := StartTimer("test_operation")
	time.Sleep(10 * time.Millisecond)
	timer.Stop()

	value := StorageOperationLatency.Get("test_operation")
	if value == nil {
		t.Fatal("timer did not record metric")
	}
}

func TestPluginTimer(t *testing.T) {
	timer := StartPluginTimer("test_plugin")
	time.Sleep(10 * time.Millisecond)
	timer.Stop()

	countVal := PluginExecutionCount.Get("test_plugin")
	durationVal := PluginExecutionDuration.Get("test_plugin")

	if countVal == nil {
		t.Fatal("plugin timer did not record count")
	}
	if durationVal == nil {
		t.Fatal("plugin timer did not record duration")
	}
}

func TestAPITimer(t *testing.T) {
	timer := StartAPITimer("/test/endpoint")
	time.Sleep(10 * time.Millisecond)
	timer.Stop()

	countVal := APIRequestCount.Get("/test/endpoint")
	durationVal := APIRequestDuration.Get("/test/endpoint")

	if countVal == nil {
		t.Fatal("API timer did not record count")
	}
	if durationVal == nil {
		t.Fatal("API timer did not record duration")
	}
}
