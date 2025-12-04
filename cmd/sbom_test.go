package cmd

import (
	"bytes"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/display"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSBOMCommandStructure(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	sbomCmd, _, err := cmd.Find([]string{"get", "sbom"})
	require.NoError(t, err, "Failed to find get sbom command")

	assert.Equal(t, "sbom <owner/package>", sbomCmd.Use)
	assert.NotEmpty(t, sbomCmd.Short, "Expected non-empty Short description")
	assert.NotEmpty(t, sbomCmd.Long, "Expected non-empty Long description")
}

func TestGetSBOMCommandArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		args      []string
		wantError bool
	}{
		{
			name:      "missing image argument",
			args:      []string{"get", "sbom"},
			wantError: true,
		},
		{
			name:      "valid single argument with owner/package",
			args:      []string{"get", "sbom", "mkoepf/test-image"},
			wantError: false, // Will fail for other reasons (no selector flag), but not arg count
		},
		{
			name:      "too many arguments",
			args:      []string{"get", "sbom", "image1", "image2"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCmd()
			var out bytes.Buffer
			var errOut bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&errOut)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if tt.wantError {
				assert.Error(t, err, "Expected error but got none")
			}
			// For valid args count, it may still fail but not due to arg count
			if !tt.wantError && err != nil {
				// Check if it's an args error
				errStr := err.Error()
				assert.NotEqual(t, "accepts 1 arg(s), received 0", errStr, "Unexpected arg count error")
				assert.NotEqual(t, "accepts 1 arg(s), received 2", errStr, "Unexpected arg count error")
			}
		})
	}
}

func TestGetSBOMCommandHasFlags(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	sbomCmd, _, _ := cmd.Find([]string{"get", "sbom"})

	// Check for required flags
	flags := []string{"tag", "digest", "version", "all", "json"}

	for _, flagName := range flags {
		flag := sbomCmd.Flags().Lookup(flagName)
		assert.NotNil(t, flag, "Expected flag '%s' to exist", flagName)
	}
}

func TestShortDigest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		digest   string
		expected string
	}{
		{
			name:     "full sha256 digest",
			digest:   "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expected: "1234567890ab",
		},
		{
			name:     "short digest",
			digest:   "sha256:abc",
			expected: "abc",
		},
		{
			name:     "no prefix",
			digest:   "1234567890abcdef",
			expected: "1234567890ab",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := display.ShortDigest(tt.digest)
			assert.Equal(t, tt.expected, result, "display.ShortDigest(%s)", tt.digest)
		})
	}
}
