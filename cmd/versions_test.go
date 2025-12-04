package cmd

import (
	"bytes"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildVersionFilter verifies that the filter is built correctly from flags
func TestBuildVersionFilter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		tag          string
		tagPattern   string
		onlyTagged   bool
		onlyUntagged bool
		olderThan    string
		newerThan    string
		versionID    int64
		digest       string
		wantErr      bool
		errContains  string
	}{
		{
			name:    "no filters",
			wantErr: false,
		},
		{
			name:         "conflicting tagged/untagged flags",
			onlyTagged:   true,
			onlyUntagged: true,
			wantErr:      true,
			errContains:  "cannot use --tagged and --untagged together",
		},
		{
			name:        "invalid older-than value",
			olderThan:   "invalid-date",
			wantErr:     true,
			errContains: "invalid --older-than value",
		},
		{
			name:      "valid older-than date RFC3339",
			olderThan: "2025-01-01T00:00:00Z",
			wantErr:   false,
		},
		{
			name:      "valid older-than date (date only)",
			olderThan: "2025-01-01",
			wantErr:   false,
		},
		{
			name:      "valid newer-than date (date only)",
			newerThan: "2025-11-01",
			wantErr:   false,
		},
		{
			name:      "valid older-than duration (days)",
			olderThan: "7d",
			wantErr:   false,
		},
		{
			name:      "valid newer-than duration (hours)",
			newerThan: "24h",
			wantErr:   false,
		},
		{
			name:      "valid duration (combined)",
			olderThan: "1h30m",
			wantErr:   false,
		},
		{
			name:      "version ID filter",
			versionID: 12345,
			wantErr:   false,
		},
		{
			name:    "digest filter",
			digest:  "sha256:abc123",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := buildListVersionFilter(
				tt.tag, tt.tagPattern, tt.onlyTagged, tt.onlyUntagged,
				tt.olderThan, tt.newerThan,
				tt.versionID, tt.digest,
			)

			if tt.wantErr {
				require.Error(t, err, "Expected error but got none")
				if tt.errContains != "" {
					assert.ErrorContains(t, err, tt.errContains)
				}
			} else {
				require.NoError(t, err, "Unexpected error")
				assert.NotNil(t, filter, "Expected non-nil filter")
			}
		})
	}
}

// TestListVersionsCommandStructure verifies the list versions command is properly set up
func TestListVersionsCommandStructure(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	versionsCmd, _, err := cmd.Find([]string{"list", "versions"})
	require.NoError(t, err, "Failed to find list versions command")

	assert.Equal(t, "versions <owner/package>", versionsCmd.Use)
	assert.NotEmpty(t, versionsCmd.Short, "Short description should not be empty")
}

// TestListVersionsCommandArguments verifies argument validation
func TestListVersionsCommandArguments(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectUsage bool
	}{
		{
			name:        "missing package argument",
			args:        []string{"list", "versions"},
			expectUsage: true,
		},
		{
			name:        "too many arguments",
			args:        []string{"list", "versions", "mypackage", "extra"},
			expectUsage: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCmd()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			// Should fail with usage error
			assert.Error(t, err, "Expected error but got none")
		})
	}
}

// TestListVersionsCommandHasFlags verifies required flags exist
func TestListVersionsCommandHasFlags(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	versionsCmd, _, _ := cmd.Find([]string{"list", "versions"})

	requiredFlags := []string{
		"json",
		"tag",
		"tag-pattern",
		"tagged",
		"untagged",
		"older-than",
		"newer-than",
		"version",
		"digest",
	}

	for _, flagName := range requiredFlags {
		flag := versionsCmd.Flags().Lookup(flagName)
		assert.NotNil(t, flag, "list versions command should have --%s flag", flagName)
	}

	// Verify removed flags are gone
	removedFlags := []string{"older-than-days", "newer-than-days"}
	for _, flagName := range removedFlags {
		flag := versionsCmd.Flags().Lookup(flagName)
		assert.Nil(t, flag, "list versions command should NOT have --%s flag (use duration with --older-than/--newer-than instead)", flagName)
	}
}

// TestListVersionsCommandNoTreeFlag verifies --tree flag was removed
// (use 'ghcrctl list graphs' for tree view instead)
func TestListVersionsCommandNoTreeFlag(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	versionsCmd, _, _ := cmd.Find([]string{"list", "versions"})

	flag := versionsCmd.Flags().Lookup("tree")
	assert.Nil(t, flag, "list versions command should NOT have --tree flag (use 'ghcrctl list graphs' instead)")
}

// TestOutputListVersionsTableQuietMode verifies quiet mode suppresses informational output
func TestOutputListVersionsTableQuietMode(t *testing.T) {
	t.Parallel()
	versions := []gh.PackageVersionInfo{
		{ID: 123, Digest: "sha256:abc123", Tags: []string{"v1.0.0"}, CreatedAt: "2025-01-01"},
	}

	// Normal mode should include header and summary
	var normalBuf bytes.Buffer
	err := OutputVersionsTable(&normalBuf, versions, "testpkg", false)
	require.NoError(t, err, "unexpected error")
	normalOutput := normalBuf.String()
	assert.Contains(t, normalOutput, "Versions for testpkg", "normal mode should include 'Versions for' header")
	assert.Contains(t, normalOutput, "Total:", "normal mode should include 'Total:' summary")

	// Quiet mode should NOT include header or summary
	var quietBuf bytes.Buffer
	err = OutputVersionsTable(&quietBuf, versions, "testpkg", true)
	require.NoError(t, err, "unexpected error")
	quietOutput := quietBuf.String()
	assert.NotContains(t, quietOutput, "Versions for testpkg", "quiet mode should NOT include 'Versions for' header")
	assert.NotContains(t, quietOutput, "Total:", "quiet mode should NOT include 'Total:' summary")
	// But should still have data
	assert.Contains(t, quietOutput, "123", "quiet mode should still include version ID")
}
