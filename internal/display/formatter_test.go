package display

import (
	"bytes"
	"strings"
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

func TestOutputJSON(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		expected string
		wantErr  bool
	}{
		{
			name:     "simple string slice",
			data:     []string{"a", "b", "c"},
			expected: "[\n  \"a\",\n  \"b\",\n  \"c\"\n]\n",
			wantErr:  false,
		},
		{
			name:     "map",
			data:     map[string]string{"key": "value"},
			expected: "{\n  \"key\": \"value\"\n}\n",
			wantErr:  false,
		},
		{
			name: "struct",
			data: struct {
				Name string `json:"name"`
				Age  int    `json:"age"`
			}{Name: "test", Age: 25},
			expected: "{\n  \"name\": \"test\",\n  \"age\": 25\n}\n",
			wantErr:  false,
		},
		{
			name:     "nil",
			data:     nil,
			expected: "null\n",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := OutputJSON(&buf, tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("OutputJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			got := buf.String()
			if got != tt.expected {
				t.Errorf("OutputJSON() output = %q, expected %q", got, tt.expected)
			}
		})
	}
}

func TestOutputJSONWithInvalidData(t *testing.T) {
	var buf bytes.Buffer
	// Functions cannot be marshaled to JSON
	err := OutputJSON(&buf, func() {})
	if err == nil {
		t.Error("OutputJSON() should return error for unmarshalable data")
	}
	if !strings.Contains(err.Error(), "failed to marshal JSON") {
		t.Errorf("OutputJSON() error should mention JSON marshaling, got: %v", err)
	}
}
