package prompts

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfirm_EOF(t *testing.T) {
	t.Parallel()

	// Empty reader simulates EOF (closed stdin)
	reader := strings.NewReader("")
	writer := &bytes.Buffer{}

	result, err := Confirm(reader, writer, "Test prompt")
	require.NoError(t, err, "EOF should not return error")
	assert.False(t, result, "EOF should default to no (false)")
}

func TestConfirmWithInput(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedValue  string
		expectedResult bool
	}{
		{
			name:           "exact match",
			input:          "myimage\n",
			expectedValue:  "myimage",
			expectedResult: true,
		},
		{
			name:           "match with whitespace",
			input:          "  myimage  \n",
			expectedValue:  "myimage",
			expectedResult: true,
		},
		{
			name:           "wrong input",
			input:          "wrongname\n",
			expectedValue:  "myimage",
			expectedResult: false,
		},
		{
			name:           "empty input",
			input:          "\n",
			expectedValue:  "myimage",
			expectedResult: false,
		},
		{
			name:           "partial match fails",
			input:          "myimag\n",
			expectedValue:  "myimage",
			expectedResult: false,
		},
		{
			name:           "case sensitive",
			input:          "MyImage\n",
			expectedValue:  "myimage",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			writer := &bytes.Buffer{}

			result, err := ConfirmWithInput(reader, writer, "Type the name to confirm", tt.expectedValue)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)

			// Verify prompt was written
			output := writer.String()
			assert.Contains(t, output, "Type the name to confirm")
		})
	}
}

func TestConfirmWithInput_EOF(t *testing.T) {
	t.Parallel()

	reader := strings.NewReader("")
	writer := &bytes.Buffer{}

	result, err := ConfirmWithInput(reader, writer, "Type to confirm", "myimage")
	require.NoError(t, err, "EOF should not return error")
	assert.False(t, result, "EOF should default to no (false)")
}

func TestConfirm(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "yes response",
			input:    "y\n",
			expected: true,
		},
		{
			name:     "yes full word",
			input:    "yes\n",
			expected: true,
		},
		{
			name:     "YES uppercase",
			input:    "YES\n",
			expected: true,
		},
		{
			name:     "no response",
			input:    "n\n",
			expected: false,
		},
		{
			name:     "no full word",
			input:    "no\n",
			expected: false,
		},
		{
			name:     "empty input defaults to no",
			input:    "\n",
			expected: false,
		},
		{
			name:     "random text defaults to no",
			input:    "maybe\n",
			expected: false,
		},
		{
			name:     "whitespace around yes",
			input:    "  yes  \n",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			writer := &bytes.Buffer{}

			result, err := Confirm(reader, writer, "Test prompt")
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)

			// Verify prompt was written
			output := writer.String()
			assert.Contains(t, output, "Test prompt")
			assert.Contains(t, output, "[y/N]")
		})
	}
}
