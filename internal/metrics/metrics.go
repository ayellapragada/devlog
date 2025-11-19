package metrics

import (
	"expvar"
	"sync"
	"time"
)

var (
	EventIngestionRate      = expvar.NewInt("events.ingested.total")
	EventIngestionErrors    = expvar.NewInt("events.ingested.errors")
	StorageOperationLatency = expvar.NewMap("storage.operation.latency_ms")
	PluginExecutionCount    = expvar.NewMap("plugins.execution.count")
	PluginExecutionDuration = expvar.NewMap("plugins.execution.duration_ms")
	APIRequestCount         = expvar.NewMap("api.requests.count")
	APIRequestDuration      = expvar.NewMap("api.requests.duration_ms")
)

type Timer struct {
	start time.Time
	name  string
}

func StartTimer(name string) *Timer {
	return &Timer{
		start: time.Now(),
		name:  name,
	}
}

func (t *Timer) Stop() {
	duration := time.Since(t.start)
	StorageOperationLatency.Add(t.name, duration.Milliseconds())
}

type PluginTimer struct {
	start time.Time
	name  string
}

func StartPluginTimer(pluginName string) *PluginTimer {
	return &PluginTimer{
		start: time.Now(),
		name:  pluginName,
	}
}

func (t *PluginTimer) Stop() {
	duration := time.Since(t.start)
	PluginExecutionCount.Add(t.name, 1)
	PluginExecutionDuration.Add(t.name, duration.Milliseconds())
}

type APITimer struct {
	start    time.Time
	endpoint string
}

func StartAPITimer(endpoint string) *APITimer {
	return &APITimer{
		start:    time.Now(),
		endpoint: endpoint,
	}
}

func (t *APITimer) Stop() {
	duration := time.Since(t.start)
	APIRequestCount.Add(t.endpoint, 1)
	APIRequestDuration.Add(t.endpoint, duration.Milliseconds())
}

type Counter struct {
	mu    sync.Mutex
	value int64
}

func (c *Counter) Add(delta int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value += delta
}

func (c *Counter) Value() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

func (c *Counter) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	i := expvar.NewInt("")
	i.Set(c.value)
	return i.String()
}
