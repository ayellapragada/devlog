package summarizer

import (
	"fmt"
	"sort"
	"strings"

	"devlog/internal/events"
)

type repoActivity struct {
	Repo         string
	Branch       string
	EventCount   int
	CriticalHigh int
	MediumLow    int
}

func extractRepoActivity(evts []*events.Event) []repoActivity {
	type key struct {
		repo   string
		branch string
	}

	activityMap := make(map[key]*repoActivity)

	sourcePriority := map[string]int{
		"claude":    3,
		"github":    2,
		"git":       1,
		"kubectl":   1,
		"shell":     0,
		"clipboard": 0,
	}

	for _, evt := range evts {
		if evt.Repo == "" {
			continue
		}

		k := key{repo: evt.Repo, branch: evt.Branch}
		if activityMap[k] == nil {
			activityMap[k] = &repoActivity{
				Repo:   evt.Repo,
				Branch: evt.Branch,
			}
		}

		activityMap[k].EventCount++

		priority := sourcePriority[evt.Source]
		if priority >= 2 {
			activityMap[k].CriticalHigh++
		} else {
			activityMap[k].MediumLow++
		}
	}

	activities := make([]repoActivity, 0, len(activityMap))
	for _, activity := range activityMap {
		activities = append(activities, *activity)
	}

	sort.Slice(activities, func(i, j int) bool {
		if activities[i].CriticalHigh != activities[j].CriticalHigh {
			return activities[i].CriticalHigh > activities[j].CriticalHigh
		}
		if activities[i].MediumLow != activities[j].MediumLow {
			return activities[i].MediumLow > activities[j].MediumLow
		}
		return activities[i].EventCount > activities[j].EventCount
	})

	return activities
}

func buildPrompt(contextEvents, focusEvents []*events.Event, formatter func(*events.Event) string) string {
	contextBySource := groupEventsBySource(contextEvents)
	focusBySource := groupEventsBySource(focusEvents)

	repoActivities := extractRepoActivity(focusEvents)
	repoSection := ""
	if len(repoActivities) > 0 {
		repoSection = "\nACTIVE REPOSITORIES IN FOCUS PERIOD:\n"
		for _, activity := range repoActivities {
			branchInfo := ""
			if activity.Branch != "" {
				branchInfo = fmt.Sprintf(" (%s)", activity.Branch)
			}
			repoSection += fmt.Sprintf("- %s%s: %d events (%d CRITICAL/HIGH, %d MEDIUM/LOW)\n",
				activity.Repo, branchInfo, activity.EventCount, activity.CriticalHigh, activity.MediumLow)
		}
		repoSection += "\n"
	}

	prompt := `You are generating a factual development summary. This is a deterministic
transformation of the provided events, not a creative task. You must ONLY use
information explicitly present in the events. Never guess, infer intent, or
invent missing details.

You will be given two sets of events:

1. CONTEXT EVENTS — older events for background reference only
2. FOCUS EVENTS — the period that MUST be summarized

Events are grouped by source category:
- CRITICAL: Claude Code conversations, major architectural work
- HIGH: GitHub commits, PR activity
- MEDIUM: git commands, kubectl operations
- LOW: shell commands, clipboard activity, misc background
` + repoSection + `
CONTEXT EVENTS (read for background only; DO NOT summarize these):
` + formattedBySource(contextBySource, formatter) + `

FOCUS EVENTS (summarize ONLY these):
` + formattedBySource(focusBySource, formatter) + `

==================== SUMMARY REQUIREMENTS ====================

Your output has exactly two parts:

----------------------------------------------------------------
PART 1 — CONTEXT LINE (one line, max 80 chars)

Format:
Single repo: "Working on: <repo> (<branch>)"
Multiple repos (2-3): "Working on: <repo1> (<branch1>), <repo2> (<branch2>)"
Many repos (4+): "Working on: <repo1> (<branch1>) + N other repos"

Rules:
- Use the ACTIVE REPOSITORIES section above for repo/branch information
- List repos in priority order (already sorted by CRITICAL/HIGH activity)
- If no repo/branch: use "Working on: <primary-topic>"
- Never use asterisks or markdown formatting in the context line
- Keep concise: if listing multiple repos would exceed 80 chars, use "+ N other repos" format
----------------------------------------------------------------

PART 2 — ACTIVITY SUMMARY (2–4 bullet points)

Each bullet MUST:
- Be one complete sentence in past tense
- Start with a strong action verb (not "Implemented clipboard operations" but specific action)
- Include technical specifics: file paths, function names, tool names, error messages
- Consolidate repetitive actions into patterns
- Focus on what was accomplished, not what was attempted

==================== SPECIFICITY GUIDELINES ====================

LEVEL OF DETAIL (aim for the middle):

TOO VAGUE ❌:
- "Implemented clipboard copy operations throughout the session"
- "Discussed and planned model testing for summarizer plugin"
- "Ran multiple terraform plans to manage AWS infrastructure"

TOO DETAILED ❌:
- "Executed terraform plan at 11:41:03, 11:41:19, 11:41:45, and 11:41:58"
- "Ran ./scripts/benchmark_summarizer.sh at 2025-11-20 04:28:39 and 13:47:21"
- "Copied various output logs and configurations related to script testing"

JUST RIGHT ✅:
- "Created benchmark script for testing LLM models on summarizer prompt variants"
- "Debugged terraform lock issue using force-unlock, then validated infrastructure plan"
- "Evaluated qwen2.5:14b and llama3.1:8b for summarization quality and speed"

==================== CONSOLIDATION RULES ====================

You MUST consolidate repetitive or similar events:
- If >3 related operations → describe the goal, not each operation
- Repetitive debugging → "Debugged <specific-issue>" with outcome if known
- Multiple commands for same goal → one bullet describing the objective
- Clipboard/shell spam → OMIT unless it reveals important pattern

EXAMPLES:
- NOT: "Ran benchmark script twice"
- YES: "Benchmarked multiple LLM models for summarizer performance"

- NOT: "Addressed Terraform lock issues by unlocking specific resource"
- YES: "Resolved terraform state lock conflict in aws-accounts-infra"

==================== PRIORITIZATION (STRICT ORDER) ====================

1. CRITICAL: architectural decisions, major code discussions
2. HIGH: commits, PRs, major git operations
3. MEDIUM: include ONLY if needed for understanding CRITICAL/HIGH
4. LOW: include only if pattern reveals clear intent

If a lower-priority event does not add value to understanding what was accomplished, OMIT IT.

==================== HARD RULES (DO NOT BREAK THESE) ====================

NEVER use:
- "the user", "I", "we", "they"
- Uncertainty: "appears", "seems", "probably", "likely"
- Meta phrases: "worked on", "focused on", "spent time", "continued to"
- Vague actions: "made changes", "updated files", "ran commands"
- Timestamps in bullets (dates are already in event format)
- Generic accomplishments without specifics

ALWAYS use:
- Past tense action verbs
- Specific file paths when relevant
- Tool/command names when they identify the work
- Technical terminology appropriate to the domain
- Concrete outcomes when visible in events

==================== GOOD OUTPUT EXAMPLES ====================

GOOD:
Working on: devlog (main)

- Created benchmark_summarizer.sh to test qwen and llama models with different prompt variants
- Implemented automatic model unloading after tests to prevent memory exhaustion
- Fixed timestamp query bug in SQLite event fetching using unixepoch conversion

GOOD:
Working on: aws-accounts-infra (main)

- Resolved terraform state lock in wistia-dev workspace using force-unlock
- Validated infrastructure plan for ECS service updates and RDS parameter changes
- Applied terraform changes to staging environment

GOOD (mixed repos):
Working on: devlog (main)

- Discussed implementing priority-based event categorization in internal/events/event.go
- Benchmarked qwen2.5:14b for production summarizer with 50-event test cases
- Deployed configuration updates to kubernetes staging cluster

==================== OUTPUT FORMAT (STRICT) ====================

<one-line context>

- <bullet 1: most significant technical work>
- <bullet 2: second most significant work>
- <bullet 3: additional work if meaningfully different>
- <bullet 4: only if truly distinct from above>

Generate the summary now. Follow ALL rules above with zero deviations.`

	return prompt
}

func groupEventsBySource(evts []*events.Event) map[string][]*events.Event {
	grouped := make(map[string][]*events.Event)
	for _, evt := range evts {
		grouped[evt.Source] = append(grouped[evt.Source], evt)
	}
	return grouped
}

func formattedBySource(eventsBySource map[string][]*events.Event, formatter func(*events.Event) string) string {
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
			sb.WriteString(formatter(evt) + "\n")
		}
	}

	return sb.String()
}

func BuildPromptExported(contextEvents, focusEvents []*events.Event) string {
	return buildPrompt(contextEvents, focusEvents, FormatEvent)
}
