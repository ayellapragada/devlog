package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

const (
	MaxSourceTypes     = 100
	MaxEventTypes      = 100
	HourlyBucketsTTL   = 168
	DailyBucketsTTL    = 30
	RingBufferSize     = 10000
	RingBufferCleanAge = 24 * time.Hour
)

type EventRecord struct {
	Source    string
	EventType string
	Timestamp time.Time
}

type RingBuffer struct {
	mu      sync.RWMutex
	events  []EventRecord
	head    int
	tail    int
	size    int
	maxSize int
}

func NewRingBuffer(maxSize int) *RingBuffer {
	return &RingBuffer{
		events:  make([]EventRecord, maxSize),
		maxSize: maxSize,
	}
}

func (rb *RingBuffer) Add(source, eventType string, timestamp time.Time) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.events[rb.head] = EventRecord{
		Source:    source,
		EventType: eventType,
		Timestamp: timestamp,
	}

	rb.head = (rb.head + 1) % rb.maxSize

	if rb.size < rb.maxSize {
		rb.size++
	} else {
		rb.tail = (rb.tail + 1) % rb.maxSize
	}
}

func (rb *RingBuffer) GetRecent(maxAge time.Duration) []EventRecord {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.size == 0 {
		return nil
	}

	cutoff := time.Now().Add(-maxAge)
	result := make([]EventRecord, 0, rb.size)

	idx := rb.tail
	for i := 0; i < rb.size; i++ {
		event := rb.events[idx]
		if event.Timestamp.After(cutoff) {
			result = append(result, event)
		}
		idx = (idx + 1) % rb.maxSize
	}

	return result
}

func (rb *RingBuffer) Count() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size
}

type TimeBucket struct {
	EventsBySource map[string]int64 `json:"events_by_source"`
	EventsByType   map[string]int64 `json:"events_by_type"`
	TotalEvents    int64            `json:"total_events"`
}

type Snapshot struct {
	mu sync.RWMutex

	PluginStartTime  map[string]time.Time `json:"plugin_start_time"`
	PluginLastError  map[string]string    `json:"plugin_last_error"`
	PluginErrorCount map[string]int64     `json:"plugin_error_count"`
	PluginRestarts   map[string]int64     `json:"plugin_restarts"`

	EventsIngested int64            `json:"events_ingested"`
	EventsBySource map[string]int64 `json:"events_by_source"`
	EventsByType   map[string]int64 `json:"events_by_type"`

	HourlyBuckets map[int64]*TimeBucket `json:"hourly_buckets,omitempty"`
	DailyBuckets  map[int64]*TimeBucket `json:"daily_buckets,omitempty"`

	QueueDepth   int64 `json:"queue_depth"`
	DatabaseSize int64 `json:"database_size_bytes"`
	EventCount   int64 `json:"event_count"`

	UptimeSeconds int64     `json:"uptime_seconds"`
	LastStartTime time.Time `json:"last_start_time"`

	ringBuffer  *RingBuffer
	lastCleanup time.Time
}

var GlobalSnapshot = NewSnapshot()

func NewSnapshot() *Snapshot {
	return &Snapshot{
		PluginStartTime:  make(map[string]time.Time),
		PluginLastError:  make(map[string]string),
		PluginErrorCount: make(map[string]int64),
		PluginRestarts:   make(map[string]int64),
		EventsBySource:   make(map[string]int64),
		EventsByType:     make(map[string]int64),
		HourlyBuckets:    make(map[int64]*TimeBucket),
		DailyBuckets:     make(map[int64]*TimeBucket),
		LastStartTime:    time.Now(),
		ringBuffer:       NewRingBuffer(RingBufferSize),
		lastCleanup:      time.Now(),
	}
}

func (s *Snapshot) RecordPluginStart(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PluginStartTime[name] = time.Now()
}

func (s *Snapshot) RecordPluginError(name string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PluginErrorCount[name]++
	s.PluginLastError[name] = err.Error()
}

func (s *Snapshot) RecordPluginRestart(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PluginRestarts[name]++
}

func (s *Snapshot) RecordEventIngested(source, eventType string) {
	now := time.Now()

	s.ringBuffer.Add(source, eventType, now)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.EventsIngested++

	if len(s.EventsBySource) < MaxSourceTypes {
		s.EventsBySource[source]++
	} else if _, exists := s.EventsBySource[source]; exists {
		s.EventsBySource[source]++
	}

	if len(s.EventsByType) < MaxEventTypes {
		s.EventsByType[eventType]++
	} else if _, exists := s.EventsByType[eventType]; exists {
		s.EventsByType[eventType]++
	}

	hourKey := now.Unix() / 3600
	dayKey := now.Unix() / 86400

	if bucket, ok := s.HourlyBuckets[hourKey]; ok {
		bucket.TotalEvents++
		bucket.EventsBySource[source]++
		bucket.EventsByType[eventType]++
	} else {
		s.HourlyBuckets[hourKey] = &TimeBucket{
			EventsBySource: map[string]int64{source: 1},
			EventsByType:   map[string]int64{eventType: 1},
			TotalEvents:    1,
		}
	}

	if bucket, ok := s.DailyBuckets[dayKey]; ok {
		bucket.TotalEvents++
		bucket.EventsBySource[source]++
		bucket.EventsByType[eventType]++
	} else {
		s.DailyBuckets[dayKey] = &TimeBucket{
			EventsBySource: map[string]int64{source: 1},
			EventsByType:   map[string]int64{eventType: 1},
			TotalEvents:    1,
		}
	}

	if time.Since(s.lastCleanup) > time.Hour {
		s.cleanupOldBuckets()
		s.rebuildFromRingBuffer()
		s.lastCleanup = now
	}
}

func (s *Snapshot) cleanupOldBuckets() {
	now := time.Now()
	hourKey := now.Unix() / 3600
	dayKey := now.Unix() / 86400

	for k := range s.HourlyBuckets {
		if hourKey-k > HourlyBucketsTTL {
			delete(s.HourlyBuckets, k)
		}
	}

	for k := range s.DailyBuckets {
		if dayKey-k > DailyBucketsTTL {
			delete(s.DailyBuckets, k)
		}
	}
}

func (s *Snapshot) rebuildFromRingBuffer() {
	recentEvents := s.ringBuffer.GetRecent(RingBufferCleanAge)

	s.EventsBySource = make(map[string]int64)
	s.EventsByType = make(map[string]int64)

	for _, event := range recentEvents {
		if len(s.EventsBySource) < MaxSourceTypes {
			s.EventsBySource[event.Source]++
		} else if _, exists := s.EventsBySource[event.Source]; exists {
			s.EventsBySource[event.Source]++
		}

		if len(s.EventsByType) < MaxEventTypes {
			s.EventsByType[event.EventType]++
		} else if _, exists := s.EventsByType[event.EventType]; exists {
			s.EventsByType[event.EventType]++
		}
	}
}

func (s *Snapshot) UpdateSystemMetrics(queueDepth, dbSize, eventCount int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.QueueDepth = queueDepth
	s.DatabaseSize = dbSize
	s.EventCount = eventCount
	s.UptimeSeconds = int64(time.Since(s.LastStartTime).Seconds())
}

func (s *Snapshot) Copy() *Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := &Snapshot{
		PluginStartTime:  make(map[string]time.Time),
		PluginLastError:  make(map[string]string),
		PluginErrorCount: make(map[string]int64),
		PluginRestarts:   make(map[string]int64),
		EventsBySource:   make(map[string]int64),
		EventsByType:     make(map[string]int64),
		HourlyBuckets:    make(map[int64]*TimeBucket),
		DailyBuckets:     make(map[int64]*TimeBucket),
		EventsIngested:   s.EventsIngested,
		QueueDepth:       s.QueueDepth,
		DatabaseSize:     s.DatabaseSize,
		EventCount:       s.EventCount,
		UptimeSeconds:    s.UptimeSeconds,
		LastStartTime:    s.LastStartTime,
		ringBuffer:       s.ringBuffer,
		lastCleanup:      s.lastCleanup,
	}

	for k, v := range s.PluginStartTime {
		snapshot.PluginStartTime[k] = v
	}
	for k, v := range s.PluginLastError {
		snapshot.PluginLastError[k] = v
	}
	for k, v := range s.PluginErrorCount {
		snapshot.PluginErrorCount[k] = v
	}
	for k, v := range s.PluginRestarts {
		snapshot.PluginRestarts[k] = v
	}
	for k, v := range s.EventsBySource {
		snapshot.EventsBySource[k] = v
	}
	for k, v := range s.EventsByType {
		snapshot.EventsByType[k] = v
	}
	for k, v := range s.HourlyBuckets {
		snapshot.HourlyBuckets[k] = &TimeBucket{
			EventsBySource: copyMap(v.EventsBySource),
			EventsByType:   copyMap(v.EventsByType),
			TotalEvents:    v.TotalEvents,
		}
	}
	for k, v := range s.DailyBuckets {
		snapshot.DailyBuckets[k] = &TimeBucket{
			EventsBySource: copyMap(v.EventsBySource),
			EventsByType:   copyMap(v.EventsByType),
			TotalEvents:    v.TotalEvents,
		}
	}

	return snapshot
}

func (s *Snapshot) WriteToFile(path string) error {
	snapshot := s.Copy()

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (s *Snapshot) ToJSON() ([]byte, error) {
	snapshot := s.Copy()
	return json.MarshalIndent(snapshot, "", "  ")
}

type Summary struct {
	Uptime         string            `json:"uptime"`
	EventCount     int64             `json:"event_count"`
	QueueDepth     int64             `json:"queue_depth"`
	EventsBySource map[string]int64  `json:"events_by_source"`
	PluginStatus   map[string]string `json:"plugin_status"`
	ErrorCount     int64             `json:"total_errors"`
}

func (s *Snapshot) GetSummary() *Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	uptime := time.Since(s.LastStartTime)
	uptimeStr := formatDuration(uptime)

	pluginStatus := make(map[string]string)
	var totalErrors int64

	for name, startTime := range s.PluginStartTime {
		runtime := time.Since(startTime)
		errorCount := s.PluginErrorCount[name]
		totalErrors += errorCount

		if errorCount > 0 {
			pluginStatus[name] = "running (with errors)"
		} else {
			pluginStatus[name] = "running"
		}

		if runtime < 5*time.Second {
			pluginStatus[name] = "starting"
		}
	}

	return &Summary{
		Uptime:         uptimeStr,
		EventCount:     s.EventCount,
		QueueDepth:     s.QueueDepth,
		EventsBySource: copyMap(s.EventsBySource),
		PluginStatus:   pluginStatus,
		ErrorCount:     totalErrors,
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "< 1m"
	}
	if d < time.Hour {
		return d.Round(time.Minute).String()
	}
	if d < 24*time.Hour {
		return d.Round(time.Minute).String()
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd%dh", days, hours)
}

func copyMap[K comparable, V any](m map[K]V) map[K]V {
	result := make(map[K]V)
	for k, v := range m {
		result[k] = v
	}
	return result
}
