package clipboard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"devlog/internal/events"

	"golang.design/x/clipboard"
)

type Poller struct {
	dataDir      string
	pollInterval time.Duration
	maxLength    int
	minLength    int
	lastHash     string
}

func NewPoller(dataDir string, pollInterval time.Duration, maxLength, minLength int) *Poller {
	return &Poller{
		dataDir:      dataDir,
		pollInterval: pollInterval,
		maxLength:    maxLength,
		minLength:    minLength,
	}
}

func (p *Poller) Name() string {
	return "clipboard"
}

func (p *Poller) PollInterval() time.Duration {
	return p.pollInterval
}

func (p *Poller) Poll(ctx context.Context) ([]*events.Event, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	content := clipboard.Read(clipboard.FmtText)
	if len(content) == 0 {
		return nil, nil
	}

	text := string(content)

	if len(text) < p.minLength {
		return nil, nil
	}

	if p.maxLength > 0 && len(text) > p.maxLength {
		text = text[:p.maxLength]
	}

	hash := hashContent(text)
	if hash == p.lastHash {
		return nil, nil
	}

	p.lastHash = hash

	fmt.Printf("Clipboard: detected new content (%d bytes)\n", len(text))

	event := events.NewEvent("clipboard", "copy")
	event.Timestamp = time.Now().Format(time.RFC3339)
	event.Payload = map[string]interface{}{
		"content": text,
		"length":  len(text),
	}

	return []*events.Event{event}, nil
}

func (p *Poller) ShouldRetryOnError(err error) bool {
	return true
}

func hashContent(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}

func (p *Poller) Init() error {
	if err := clipboard.Init(); err != nil {
		return fmt.Errorf("initialize clipboard: %w", err)
	}

	stateFile := filepath.Join(p.dataDir, "clipboard_state")
	if data, err := os.ReadFile(stateFile); err == nil {
		p.lastHash = string(data)
	}

	return nil
}

func (p *Poller) saveState() error {
	stateFile := filepath.Join(p.dataDir, "clipboard_state")
	return os.WriteFile(stateFile, []byte(p.lastHash), 0644)
}
