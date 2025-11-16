# DevLog — Product Roadmap

This roadmap tracks all planned features for the productivity daemon, organized by version and current implementation status.

---

## ✅ V0.1-0.2 — Foundation (COMPLETE)
**Status**: Shipped

- [x] Daemon architecture with HTTP API
- [x] SQLite event storage
- [x] CLI commands (ingest, daemon control, status)
- [x] Git hooks (post-commit, post-merge, post-checkout)
- [x] Shell command tracking (filtered important commands)
- [x] Wisprflow speech-to-text integration
- [x] Config file system (~/.config/devlog/config.yaml)

---

## 🚧 V0.3 — Session Intelligence (IN PROGRESS)
**Focus**: Smart event grouping and context building

### Core Session Logic
- [ ] Session aggregation (<45min gap, repo-aware grouping)
- [ ] Session state management in SQLite
- [ ] Event-to-session assignment logic
- [ ] Session closure detection (idle time)

### Enhanced Event Capture
- [ ] Clipboard history tracking (deduped + timestamped)
- [ ] Long-running command detection and tracking
- [ ] Git operations expansion (branch switch, merge, rebase, stash)
- [ ] Periodic repo scanning for missed commits

---

## 🎯 V0.4 — LLM Summarization & Output
**Focus**: Intelligent summaries and Obsidian integration

### Summarization Engine
- [ ] LLM provider abstraction (Anthropic Claude)
- [ ] Session summary generation (every 30 min)
- [ ] Daily summary generation (18:30 local)
- [ ] Prompt templates for different event types
- [ ] Summary quality metrics

### Obsidian Integration
- [ ] Markdown file generation (YYYY-MM-DD.md format)
- [ ] Captain's Log section formatting
- [ ] File path resolution for Obsidian vault
- [ ] Atomic writes to prevent corruption
- [ ] Backfill support for historical dates

---

## 🔍 V0.5 — Context Awareness
**Focus**: Understanding what the developer is doing

### Application Tracking
- [ ] App/window focus tracking (editor, terminal, browser)
- [ ] Browser tab title capture (no URLs)
- [ ] Active application dwell time
- [ ] Context switch detection between apps

### Development Environment
- [ ] VS Code/Cursor activity tracking
  - [ ] Workspace changes
  - [ ] Active file tracking
  - [ ] Extension event hooks
- [ ] Tmux session + pane switching detection
- [ ] Terminal multiplexer awareness

### Repository Intelligence
- [ ] Repo enter/exit events
- [ ] Repo dwell time tracking
- [ ] File touch tracking (which files were modified)
- [ ] Working directory changes

---

## 🧠 V0.6 — High-Signal Coding Activity
**Focus**: Capture meaningful coding work, not just keystrokes

### Code Activity Detection
- [ ] "Coding burst" detection (keystroke intensity + file types)
- [ ] File creation/modification events
- [ ] Script execution tracking (migrations, infra files)
- [ ] Code compilation events

### Development Workflow
- [ ] Local dev server interaction tracking
  - [ ] HTTP method/status/duration for localhost requests
  - [ ] Server start/stop events
  - [ ] Hot reload cycles
- [ ] Test execution tracking
  - [ ] Test framework detection (go test, rspec, jest, pytest)
  - [ ] Test run duration
  - [ ] Pass/fail status
  - [ ] Test file modifications

### Build & Deploy
- [ ] Docker build duration + outcome tracking
- [ ] Build system events (make, bazel, etc.)
- [ ] Local deployment events

---

## 🎛️ V0.7 — Infrastructure & DevOps Signals
**Focus**: Cloud, containers, and infrastructure work

### Kubernetes
- [ ] kubectl context changes
- [ ] Namespace switching
- [ ] Pod/deployment operations
- [ ] kubectl exec sessions

### Infrastructure as Code
- [ ] Terraform apply/plan events + durations
- [ ] Terraform state changes
- [ ] Cloud provider CLI commands (aws, gcp, azure)

### Container Operations
- [ ] Docker build tracking (already in V0.6)
- [ ] Docker compose up/down events
- [ ] Container lifecycle events

---

## 🧩 V0.8 — Cognitive Load & Context Switching
**Focus**: Understanding mental overhead and interruptions

### Context Switch Detection
- [ ] Application switching frequency
- [ ] Tab switching in browsers
- [ ] Terminal switching
- [ ] Repository switching

### Cognitive Load Indicators
- [ ] Multi-tasking detection
- [ ] Rapid context switches (< 2 min)
- [ ] Parallel work tracking (multiple terminals/editors)
- [ ] "Thrashing" detection (excessive switching)

### AI Integration Tracking
- [ ] AI session grouping
- [ ] Paste events after AI messages
- [ ] AI-assisted coding detection
- [ ] LLM conversation threading

---

## 📊 V0.9 — System Health Signals
**Focus**: Environmental factors affecting productivity

### Error Detection
- [ ] Stack trace detection in terminal output
- [ ] Crash signature identification
- [ ] Error log parsing
- [ ] Panic/exception detection

### Resource Monitoring
- [ ] CPU spike detection (builds, server boots)
- [ ] Memory pressure detection
- [ ] Disk I/O spikes
- [ ] Network activity from dev tools

### Process Tracking
- [ ] Long-running background process detection
- [ ] Zombie process identification
- [ ] Process crash/restart events

---

## 🎨 V1.0 — Manual Controls & Metadata
**Focus**: User-driven event capture

### Manual Input
- [ ] Hotkey-based "start task / end task"
- [ ] Manual note injection via CLI
- [ ] Task tagging system
- [ ] Custom event types

### Automatic Boundaries
- [ ] Break detection (no input for X minutes)
- [ ] Work session boundaries
- [ ] Meeting detection
  - [ ] Zoom/Meet active + mic on
  - [ ] Calendar integration (optional)
- [ ] "Focus mode on/off" API

---

## 📈 V1.1 — Analytics & Reflection
**Focus**: Insights from captured data

### Timeline Views
- [ ] Daily timeline (chronological event stream)
- [ ] Session-based view
- [ ] Repository-based view
- [ ] Weekly/monthly aggregations

### Summaries
- [ ] Session summaries (coding, debugging, infra, writing)
- [ ] Topic-based session clustering
- [ ] Automatic categorization (feature work, bug fixes, refactoring)
- [ ] Work pattern analysis

### Periodic Reviews
- [ ] Weekly summary generation
- [ ] Monthly summary generation
- [ ] Custom date range summaries
- [ ] Trend analysis (languages, repos, activities)

---

## 🎯 V1.2 — TUI & Polish
**Focus**: Rich interactive experience

### Terminal UI
- [ ] Real-time event stream view
- [ ] Session browser
- [ ] Search and filter
- [ ] Manual event editing
- [ ] Config management UI

### Quality of Life
- [ ] Performance optimization
- [ ] Reduced battery impact
- [ ] Smart event deduplication
- [ ] Configurable sensitivity levels
- [ ] Privacy controls (exclude certain paths/commands)

---

## 🔮 Future Exploration

### Integrations
- [ ] GitHub merged PR polling
- [ ] Linear/Jira ticket correlation
- [ ] Slack status sync
- [ ] Calendar integration

### Advanced Features
- [ ] Multi-machine sync (optional)
- [ ] Team collaboration features
- [ ] Habit tracking
- [ ] Goal setting and progress
- [ ] Smart notifications

### Platform Expansion
- [ ] Linux support
- [ ] Windows support (WSL-first)

---

## Implementation Principles

1. **Privacy First**: All data stays local by default
2. **Low Overhead**: Minimal CPU/memory impact
3. **Extensible**: Plugin architecture for new event sources
4. **Configurable**: Users control what's tracked and how
5. **Transparent**: Clear logging of what's being captured
6. **Fail-Safe**: Missing events should never crash the daemon

---

## Current Focus

**Next Milestone**: V0.3 (Session Intelligence)

Priority tasks:
1. Session aggregation logic
2. Clipboard history tracking
3. Enhanced git operation tracking
4. Periodic repo scanning

See [DESIGN.md](./DESIGN.md) for architectural details.
