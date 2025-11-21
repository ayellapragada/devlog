package output

import (
	"context"
	"fmt"
	"strings"

	"devlog/internal/events"
	"devlog/internal/storage"
)

type LLMClient interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

type LLMFormatter struct {
	llmClient    LLMClient
	responseGoal string
}

func NewLLMFormatter(llmClient LLMClient, responseGoal string) ResultFormatter {
	return &LLMFormatter{
		llmClient:    llmClient,
		responseGoal: responseGoal,
	}
}

func (f *LLMFormatter) Format(ctx context.Context, results []*storage.SearchResult, query string) (string, error) {
	if len(results) == 0 {
		return "No events found matching your query.", nil
	}

	events := make([]*events.Event, len(results))
	for i, r := range results {
		events[i] = r.Event
	}

	eventsBySource := groupEventsBySource(events)

	prompt := fmt.Sprintf(`You are summarizing development activity for a user based on actual logged events.

User's question goal: %s

Events found:
 %s

Instructions:
- Provide a CONCISE, narrative summary in 1-3 paragraphs maximum
- Address the user directly using "you" when appropriate.
- Do NOT list individual events separately
- Synthesize related events into coherent narratives (e.g., "worked on authentication" instead of listing individual file edits)
- Include key details: repos, branches, significant commits, important commands
- Use past tense for completed actions
- Prioritize CRITICAL and HIGH priority events but mention other significant activity
- If there are many similar events (e.g., clipboard commands), consolidate them into a single sentence.
- Focus on what was accomplished, not individual timestamps
- Remember: the user is asking about THEIR OWN activity, so use second person ("you") not third person

Generate a concise narrative summary now.`, f.responseGoal, formattedBySource(eventsBySource))

	answer, err := f.llmClient.Complete(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("format response: %w", err)
	}

	return strings.TrimSpace(answer), nil
}

func groupEventsBySource(evts []*events.Event) map[string][]*events.Event {
	grouped := make(map[string][]*events.Event)
	for _, evt := range evts {
		grouped[evt.Source] = append(grouped[evt.Source], evt)
	}
	return grouped
}

func formattedBySource(eventsBySource map[string][]*events.Event) string {
	var sb strings.Builder

	sources := []struct {
		name  string
		label string
	}{
		{"claude", "CRITICAL"},
		{"github", "HIGH"},
		{"git", "MEDIUM"},
		{"kubectl", "MEDIUM"},
		{"shell", "LOW"},
		{"clipboard", "LOW"},
		{"tmux", "LOW"},
		{"wisprflow", "LOW"},
		{"manual", "MEDIUM"},
	}

	for _, s := range sources {
		evts := eventsBySource[s.name]
		if len(evts) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("\n=== %s: %s (%d events) ===\n", s.label, s.name, len(evts)))
		for _, evt := range evts {
			sb.WriteString(formatEventForLLM(evt) + "\n")
		}
	}

	return sb.String()
}

func formatEventForLLM(evt *events.Event) string {
	return FormatEventLine(evt, 500, 500, 300, 200)
}
