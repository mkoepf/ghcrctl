package display

import (
	"testing"
)

func TestFormatTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		expected string
	}{
		{
			name:     "empty tags",
			tags:     []string{},
			expected: "[]",
		},
		{
			name:     "nil tags",
			tags:     nil,
			expected: "[]",
		},
		{
			name:     "single tag",
			tags:     []string{"v1.0.0"},
			expected: "[v1.0.0]",
		},
		{
			name:     "multiple tags",
			tags:     []string{"v1.0.0", "latest", "stable"},
			expected: "[v1.0.0, latest, stable]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTags(tt.tags)
			if result != tt.expected {
				t.Errorf("FormatTags(%v) = %q, expected %q", tt.tags, result, tt.expected)
			}
		})
	}
}

func TestShortDigest(t *testing.T) {
	tests := []struct {
		name     string
		digest   string
		expected string
	}{
		{
			name:     "full digest with sha256 prefix",
			digest:   "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			expected: "abcdef123456",
		},
		{
			name:     "digest without sha256 prefix",
			digest:   "abcdef1234567890abcdef1234567890",
			expected: "abcdef123456",
		},
		{
			name:     "short digest",
			digest:   "sha256:abc",
			expected: "abc",
		},
		{
			name:     "exactly 12 characters after prefix",
			digest:   "sha256:abcdef123456",
			expected: "abcdef123456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShortDigest(tt.digest)
			if result != tt.expected {
				t.Errorf("ShortDigest(%q) = %q, expected %q", tt.digest, result, tt.expected)
			}
		})
	}
}
