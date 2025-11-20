package commands

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(time.Duration) bool
	}{
		{
			name:    "valid day format",
			input:   "2d",
			wantErr: false,
			check: func(d time.Duration) bool {
				return d == 2*24*time.Hour
			},
		},
		{
			name:    "single day",
			input:   "1d",
			wantErr: false,
			check: func(d time.Duration) bool {
				return d == 24*time.Hour
			},
		},
		{
			name:    "zero days",
			input:   "0d",
			wantErr: false,
			check: func(d time.Duration) bool {
				return d == 0
			},
		},
		{
			name:    "empty days string",
			input:   "d",
			wantErr: true,
		},
		{
			name:    "negative days",
			input:   "-1d",
			wantErr: true,
		},
		{
			name:    "invalid days format",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "standard duration hours",
			input:   "2h",
			wantErr: false,
			check: func(d time.Duration) bool {
				return d == 2*time.Hour
			},
		},
		{
			name:    "standard duration minutes",
			input:   "30m",
			wantErr: false,
			check: func(d time.Duration) bool {
				return d == 30*time.Minute
			},
		},
		{
			name:    "standard duration seconds",
			input:   "45s",
			wantErr: false,
			check: func(d time.Duration) bool {
				return d == 45*time.Second
			},
		},
		{
			name:    "combined standard duration",
			input:   "1h30m",
			wantErr: false,
			check: func(d time.Duration) bool {
				return d == 1*time.Hour+30*time.Minute
			},
		},
		{
			name:    "large number of days",
			input:   "365d",
			wantErr: false,
			check: func(d time.Duration) bool {
				return d == 365*24*time.Hour
			},
		},
		{
			name:    "empty string should fail",
			input:   "",
			wantErr: true,
		},
		{
			name:    "days with text should fail",
			input:   "2days",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDuration(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseDuration(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("parseDuration(%q) unexpected error: %v", tt.input, err)
				return
			}
			if tt.check != nil && !tt.check(result) {
				t.Errorf("parseDuration(%q) = %v, check failed", tt.input, result)
			}
		})
	}
}
