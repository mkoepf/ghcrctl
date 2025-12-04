package display

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			assert.Equal(t, tt.expected, result)
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
			assert.Equal(t, tt.expected, result)
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

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, buf.String())
		})
	}
}

func TestOutputJSONWithInvalidData(t *testing.T) {
	var buf bytes.Buffer
	// Functions cannot be marshaled to JSON
	err := OutputJSON(&buf, func() {})
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to marshal JSON")
}
