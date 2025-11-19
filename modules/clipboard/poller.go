package clipboard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"devlog/internal/events"
	"devlog/internal/state"

	"golang.design/x/clipboard"
)

type Poller struct {
	pollInterval     time.Duration
	maxLength        int
	minLength        int
	recentHashes     []string
	dedupHistorySize int
	stateMgr         *state.Manager
}

func NewPoller(dataDir string, pollInterval time.Duration, maxLength, minLength, dedupHistorySize int) (*Poller, error) {
	stateMgr, err := state.NewManager(dataDir)
	if err != nil {
		return nil, fmt.Errorf("create state manager: %w", err)
	}

	if dedupHistorySize <= 0 {
		dedupHistorySize = 5
	}

	return &Poller{
		pollInterval:     pollInterval,
		maxLength:        maxLength,
		minLength:        minLength,
		dedupHistorySize: dedupHistorySize,
		recentHashes:     make([]string, 0, dedupHistorySize),
		stateMgr:         stateMgr,
	}, nil
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

	if p.isTracked(hash) {
		return nil, nil
	}

	if err := p.trackHash(hash); err != nil {
		return nil, err
	}

	event := events.NewEvent("clipboard", "copy")
	event.Timestamp = time.Now().Format(time.RFC3339)
	event.Payload = map[string]interface{}{
		"content": text,
		"length":  len(text),
	}

	return []*events.Event{event}, nil
}

func hashContent(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}

func (p *Poller) isTracked(hash string) bool {
	for _, recentHash := range p.recentHashes {
		if hash == recentHash {
			return true
		}
	}
	return false
}

func (p *Poller) trackHash(hash string) error {
	p.recentHashes = append(p.recentHashes, hash)
	if len(p.recentHashes) > p.dedupHistorySize {
		p.recentHashes = p.recentHashes[1:]
	}
	if err := p.stateMgr.Set("clipboard", "recent_hashes", p.recentHashes); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	return nil
}

func (p *Poller) Init() error {
	if err := clipboard.Init(); err != nil {
		return fmt.Errorf("initialize clipboard: %w", err)
	}

	if hashes, ok := p.stateMgr.Get("clipboard", "recent_hashes"); ok {
		if hashSlice, ok := hashes.([]interface{}); ok {
			p.recentHashes = make([]string, 0, len(hashSlice))
			for _, h := range hashSlice {
				if hashStr, ok := h.(string); ok {
					p.recentHashes = append(p.recentHashes, hashStr)
				}
			}
		}
	} else if hash, ok := p.stateMgr.GetString("clipboard", "last_hash"); ok {
		p.recentHashes = []string{hash}
	}

	content := clipboard.Read(clipboard.FmtText)
	if len(content) > 0 {
		currentHash := hashContent(string(content))
		if !p.isTracked(currentHash) {
			if err := p.trackHash(currentHash); err != nil {
				return fmt.Errorf("save initial state: %w", err)
			}
		}
	}

	return nil
}
