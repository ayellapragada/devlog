package storage

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"devlog/internal/events"
)

type SearchOptions struct {
	Query         string
	Limit         int
	PayloadFilter *PayloadFilter
	Cursor        string
	After         *time.Time
	Modules       []string
	Types         []string
	RepoPattern   string
	BranchPattern string
	SortOrder     SortOrder
}

type SortOrder string

const (
	SortByRelevance SortOrder = "relevance"
	SortByTimeDesc  SortOrder = "time_desc"
	SortByTimeAsc   SortOrder = "time_asc"
)

type PayloadFilter struct {
	JSONPath string
	Value    string
}

type SearchResult struct {
	Event      *events.Event
	Rank       float64
	NextCursor string
}

var (
	ftsSpecialChars = regexp.MustCompile(`[^\w\s*"]`)
	multipleSpaces  = regexp.MustCompile(`\s+`)
)

func encodeCursor(offset int) string {
	return base64.URLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func decodeCursor(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}

	decoded, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, fmt.Errorf("invalid cursor: %w", err)
	}

	offset, err := strconv.Atoi(string(decoded))
	if err != nil {
		return 0, fmt.Errorf("invalid cursor format: %w", err)
	}

	return offset, nil
}

func sanitizeFTSQuery(query string) string {
	if query == "*" {
		return query
	}

	cleaned := strings.TrimSpace(query)

	if strings.HasPrefix(cleaned, `"`) && strings.HasSuffix(cleaned, `"`) {
		return cleaned
	}

	cleaned = ftsSpecialChars.ReplaceAllString(cleaned, " ")
	cleaned = multipleSpaces.ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)

	if cleaned == "" {
		return "*"
	}

	return cleaned
}

func (s *Storage) Search(ctx context.Context, opts SearchOptions) ([]*SearchResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 20
	}

	offset, err := decodeCursor(opts.Cursor)
	if err != nil {
		return nil, fmt.Errorf("decode cursor: %w", err)
	}

	if opts.Query == "" {
		opts.Query = "*"
	}

	sanitizedQuery := sanitizeFTSQuery(opts.Query)
	hasFTSQuery := sanitizedQuery != "" && sanitizedQuery != "*"

	hasFilters := opts.After != nil ||
		len(opts.Modules) > 0 ||
		len(opts.Types) > 0 ||
		opts.RepoPattern != "" ||
		opts.BranchPattern != "" ||
		opts.PayloadFilter != nil

	if !hasFTSQuery && !hasFilters {
		return nil, fmt.Errorf("search requires at least one filter (module, type, repo, branch, since) or a non-empty query")
	}

	var args []interface{}
	selectFields := "e.id, e.timestamp, e.source, e.type, e.repo, e.branch, e.payload"
	if hasFTSQuery {
		selectFields += ", rank"
	}

	fromClause := "FROM events e"
	var whereClauses []string

	if hasFTSQuery {
		fromClause += " JOIN events_fts ON events_fts.rowid = e.rowid"
		whereClauses = append(whereClauses, "events_fts MATCH ?")
		args = append(args, sanitizedQuery)
	}

	if opts.After != nil {
		whereClauses = append(whereClauses, "e.timestamp >= ?")
		args = append(args, opts.After.Unix())
	}

	if len(opts.Modules) > 0 {
		placeholders := make([]string, len(opts.Modules))
		for i, source := range opts.Modules {
			placeholders[i] = "?"
			args = append(args, source)
		}
		whereClauses = append(whereClauses, fmt.Sprintf("e.source IN (%s)", strings.Join(placeholders, ",")))
	}

	if len(opts.Types) > 0 {
		placeholders := make([]string, len(opts.Types))
		for i, t := range opts.Types {
			placeholders[i] = "?"
			args = append(args, t)
		}
		whereClauses = append(whereClauses, fmt.Sprintf("e.type IN (%s)", strings.Join(placeholders, ",")))
	}

	if opts.RepoPattern != "" {
		whereClauses = append(whereClauses, "e.repo LIKE ?")
		args = append(args, "%"+opts.RepoPattern+"%")
	}

	if opts.BranchPattern != "" {
		whereClauses = append(whereClauses, "e.branch LIKE ?")
		args = append(args, "%"+opts.BranchPattern+"%")
	}

	if opts.PayloadFilter != nil {
		whereClauses = append(whereClauses, "json_extract(e.payload, ?) = ?")
		args = append(args, opts.PayloadFilter.JSONPath, opts.PayloadFilter.Value)
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	orderClause := ""
	if opts.SortOrder == "" {
		opts.SortOrder = SortByTimeAsc
	}
	switch opts.SortOrder {
	case SortByRelevance:
		if hasFTSQuery {
			orderClause = "ORDER BY rank"
		} else {
			orderClause = "ORDER BY e.timestamp DESC"
		}
	case SortByTimeDesc:
		orderClause = "ORDER BY e.timestamp DESC"
	case SortByTimeAsc:
		orderClause = "ORDER BY e.timestamp ASC"
	}

	limitClause := fmt.Sprintf("LIMIT %d OFFSET %d", opts.Limit+1, offset)

	sqlQuery := fmt.Sprintf("SELECT %s %s %s %s %s",
		selectFields, fromClause, whereClause, orderClause, limitClause)

	ctx, cancel := context.WithTimeout(ctx, DefaultQueryTimeoutLong)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("search events: %w", err)
	}
	defer rows.Close()

	var results []*SearchResult
	for rows.Next() {
		result, err := s.scanSearchResultWithFTS(rows, hasFTSQuery)
		if err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	hasMore := len(results) > opts.Limit
	if hasMore {
		results = results[:opts.Limit]
		nextOffset := offset + opts.Limit
		nextCursor := encodeCursor(nextOffset)
		for i := range results {
			results[i].NextCursor = nextCursor
		}
	}

	return results, nil
}

func (s *Storage) QueryByPayloadField(ctx context.Context, jsonPath string, value string, limit int) ([]*events.Event, error) {
	sqlQuery := `
		SELECT id, timestamp, source, type, repo, branch, payload
		FROM events
		WHERE json_extract(payload, ?) = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`

	ctx, cancel := context.WithTimeout(ctx, DefaultQueryTimeoutLong)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, sqlQuery, jsonPath, value, limit)
	if err != nil {
		return nil, fmt.Errorf("query by payload field: %w", err)
	}
	defer rows.Close()

	var result []*events.Event
	for rows.Next() {
		event, err := s.scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		result = append(result, event)
	}

	return result, rows.Err()
}

func (s *Storage) scanSearchResultWithFTS(scanner interface {
	Scan(dest ...interface{}) error
}, hasFTSQuery bool) (*SearchResult, error) {
	var event events.Event
	var payloadJSON string
	var repo, branch sql.NullString
	var timestampUnix int64
	var rank float64

	var err error
	if hasFTSQuery {
		err = scanner.Scan(
			&event.ID,
			&timestampUnix,
			&event.Source,
			&event.Type,
			&repo,
			&branch,
			&payloadJSON,
			&rank,
		)
	} else {
		err = scanner.Scan(
			&event.ID,
			&timestampUnix,
			&event.Source,
			&event.Type,
			&repo,
			&branch,
			&payloadJSON,
		)
	}
	if err != nil {
		return nil, err
	}

	event.Version = 1
	event.Timestamp = time.Unix(timestampUnix, 0).UTC().Format(time.RFC3339)

	if repo.Valid {
		event.Repo = repo.String
	}

	if branch.Valid {
		event.Branch = branch.String
	}

	restoredEvent, err := s.restoreEventPayload(&event, payloadJSON)
	if err != nil {
		return nil, fmt.Errorf("restore payload: %w", err)
	}

	return &SearchResult{
		Event: restoredEvent,
		Rank:  rank,
	}, nil
}
