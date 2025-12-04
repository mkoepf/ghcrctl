package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetProvenanceCommandStructure(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	provenanceCmd, _, err := cmd.Find([]string{"get", "provenance"})
	require.NoError(t, err, "Failed to find get provenance command")

	assert.Equal(t, "provenance <owner/package>", provenanceCmd.Use)
	assert.NotEmpty(t, provenanceCmd.Short, "Expected non-empty Short description")
	assert.NotEmpty(t, provenanceCmd.Long, "Expected non-empty Long description")
}

func TestGetProvenanceCommandArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		args      []string
		wantError bool
	}{
		{
			name:      "missing image argument",
			args:      []string{"get", "provenance"},
			wantError: true,
		},
		{
			name:      "valid single argument with owner/package",
			args:      []string{"get", "provenance", "mkoepf/test-image"},
			wantError: false, // Will fail for other reasons (no selector flag), but not arg count
		},
		{
			name:      "too many arguments",
			args:      []string{"get", "provenance", "image1", "image2"},
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

func TestGetProvenanceCommandHasFlags(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	provenanceCmd, _, _ := cmd.Find([]string{"get", "provenance"})

	// Check for required flags
	flags := []string{"tag", "digest", "version", "all", "json"}

	for _, flagName := range flags {
		flag := provenanceCmd.Flags().Lookup(flagName)
		assert.NotNil(t, flag, "Expected flag '%s' to exist", flagName)
	}
}

// Note: shortProvenanceDigest was removed - now using display.ShortDigest
// The functionality is tested in internal/display/formatter_test.go
