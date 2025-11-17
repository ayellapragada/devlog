package clipboard

import (
	"testing"
)

func TestHashDeduplication(t *testing.T) {
	tests := []struct {
		name           string
		historySize    int
		clipboardSeq   []string
		expectedEvents []bool
	}{
		{
			name:           "Same content twice in a row should be deduplicated",
			historySize:    5,
			clipboardSeq:   []string{"test", "test"},
			expectedEvents: []bool{true, false},
		},
		{
			name:           "Different content should create events",
			historySize:    5,
			clipboardSeq:   []string{"test1", "test2", "test3"},
			expectedEvents: []bool{true, true, true},
		},
		{
			name:           "Content returning within history window should be deduplicated",
			historySize:    5,
			clipboardSeq:   []string{"A", "B", "C", "A"},
			expectedEvents: []bool{true, true, true, false},
		},
		{
			name:           "Content returning after history window should create event",
			historySize:    3,
			clipboardSeq:   []string{"A", "B", "C", "D", "A"},
			expectedEvents: []bool{true, true, true, true, true},
		},
		{
			name:           "Wispr Flow pattern: original, transcription, original should deduplicate",
			historySize:    5,
			clipboardSeq:   []string{"original text", "transcribed text", "original text"},
			expectedEvents: []bool{true, true, false},
		},
		{
			name:           "Multiple Wispr Flow cycles",
			historySize:    5,
			clipboardSeq:   []string{"text1", "transcription1", "text1", "text2", "transcription2", "text2"},
			expectedEvents: []bool{true, true, false, true, true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recentHashes := make([]string, 0, tt.historySize)

			for i, content := range tt.clipboardSeq {
				hash := hashContent(content)

				isDuplicate := false
				for _, recentHash := range recentHashes {
					if hash == recentHash {
						isDuplicate = true
						break
					}
				}

				shouldCreateEvent := !isDuplicate
				if shouldCreateEvent != tt.expectedEvents[i] {
					t.Errorf("Clipboard[%d] (%q): expected event=%v, got event=%v",
						i, content, tt.expectedEvents[i], shouldCreateEvent)
				}

				if !isDuplicate {
					recentHashes = append(recentHashes, hash)
					if len(recentHashes) > tt.historySize {
						recentHashes = recentHashes[1:]
					}
				}
			}
		})
	}
}

func TestHashContentConsistency(t *testing.T) {
	content := "test content"
	hash1 := hashContent(content)
	hash2 := hashContent(content)

	if hash1 != hash2 {
		t.Errorf("hashContent produced different hashes for same content: %s vs %s", hash1, hash2)
	}

	differentContent := "different content"
	hash3 := hashContent(differentContent)

	if hash1 == hash3 {
		t.Errorf("hashContent produced same hash for different content")
	}
}
