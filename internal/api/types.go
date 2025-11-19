package api

import (
	"devlog/internal/events"
)

type IngestEventRequest struct {
	Event *events.Event `json:"event"`
}

type IngestEventResponse struct {
	OK       bool   `json:"ok"`
	EventID  string `json:"event_id,omitempty"`
	Filtered bool   `json:"filtered,omitempty"`
	Error    string `json:"error,omitempty"`
}

type StatusResponse struct {
	Running       bool `json:"running"`
	EventCount    int  `json:"event_count"`
	UptimeSeconds int  `json:"uptime_seconds"`
}

type HealthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

type EventResponse struct {
	ID        string                 `json:"id"`
	Timestamp string                 `json:"timestamp"`
	Source    string                 `json:"source"`
	Type      string                 `json:"type"`
	Repo      string                 `json:"repo,omitempty"`
	Branch    string                 `json:"branch,omitempty"`
	Payload   map[string]interface{} `json:"payload"`
}

type GetEventsResponse struct {
	Events []EventResponse `json:"events"`
	Count  int             `json:"count"`
}

type SourceCount struct {
	Source string `json:"source"`
	Count  int    `json:"count"`
}

type EventsBySourceResponse struct {
	Data []SourceCount `json:"data"`
}

type TimelinePoint struct {
	Hour  string `json:"hour"`
	Count int    `json:"count"`
}

type EventsTimelineResponse struct {
	Data []TimelinePoint `json:"data"`
}

type RepoStat struct {
	Repo  string `json:"repo"`
	Count int    `json:"count"`
}

type RepoStatsResponse struct {
	Data []RepoStat `json:"data"`
}

type CommandStat struct {
	Command string `json:"command"`
	Count   int    `json:"count"`
}

type CommandStatsResponse struct {
	Data []CommandStat `json:"data"`
}

type SearchResultResponse struct {
	ID        string                 `json:"id"`
	Timestamp string                 `json:"timestamp"`
	Source    string                 `json:"source"`
	Type      string                 `json:"type"`
	Repo      string                 `json:"repo,omitempty"`
	Branch    string                 `json:"branch,omitempty"`
	Payload   map[string]interface{} `json:"payload"`
	Snippet   string                 `json:"snippet,omitempty"`
	Rank      float64                `json:"rank"`
}

type SearchResponse struct {
	Results    []SearchResultResponse `json:"results"`
	Count      int                    `json:"count"`
	Query      string                 `json:"query"`
	NextCursor string                 `json:"next_cursor,omitempty"`
	HasMore    bool                   `json:"has_more,omitempty"`
}

type ErrorResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}
