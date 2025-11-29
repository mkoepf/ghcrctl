package cmd

import (
	"testing"
)

// TestBuildVersionFilter verifies that the filter is built correctly from flags
func TestBuildVersionFilter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		tag           string
		tagPattern    string
		onlyTagged    bool
		onlyUntagged  bool
		olderThan     string
		newerThan     string
		olderThanDays int
		newerThanDays int
		versionID     int64
		digest        string
		wantErr       bool
		errContains   string
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
			name:        "invalid older-than date",
			olderThan:   "invalid-date",
			wantErr:     true,
			errContains: "invalid --older-than date format",
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
			filter, err := buildVersionFilter(
				tt.tag, tt.tagPattern, tt.onlyTagged, tt.onlyUntagged,
				tt.olderThan, tt.newerThan, tt.olderThanDays, tt.newerThanDays,
				tt.versionID, tt.digest,
			)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if filter == nil {
					t.Error("Expected non-nil filter")
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestVersionsCommandStructure verifies the versions command is properly set up
func TestVersionsCommandStructure(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	versionsCmd, _, err := cmd.Find([]string{"versions"})
	if err != nil {
		t.Fatalf("Failed to find versions command: %v", err)
	}

	if versionsCmd.Use != "versions <owner/package>" {
		t.Errorf("Expected Use 'versions <owner/package>', got '%s'", versionsCmd.Use)
	}

	if versionsCmd.Short == "" {
		t.Error("Short description should not be empty")
	}
}

// TestVersionsCommandArguments verifies argument validation
func TestVersionsCommandArguments(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectUsage bool
	}{
		{
			name:        "missing image argument",
			args:        []string{"versions"},
			expectUsage: true,
		},
		{
			name:        "too many arguments",
			args:        []string{"versions", "myimage", "extra"},
			expectUsage: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCmd()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			// Should fail with usage error
			if err == nil {
				t.Error("Expected error but got none")
			}
		})
	}
}

// TestVersionsCommandHasFlags verifies required flags exist
func TestVersionsCommandHasFlags(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	versionsCmd, _, _ := cmd.Find([]string{"versions"})

	requiredFlags := []string{
		"json",
		"tag",
		"tag-pattern",
		"tagged",
		"untagged",
		"older-than",
		"newer-than",
		"older-than-days",
		"newer-than-days",
		"version",
		"digest",
	}

	for _, flagName := range requiredFlags {
		flag := versionsCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("versions command should have --%s flag", flagName)
		}
	}
}

// TestVersionsCommandNoTreeFlag verifies --tree flag was removed
// (use 'ghcrctl images' for tree view instead)
func TestVersionsCommandNoTreeFlag(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	versionsCmd, _, _ := cmd.Find([]string{"versions"})

	flag := versionsCmd.Flags().Lookup("tree")
	if flag != nil {
		t.Error("versions command should NOT have --tree flag (use 'ghcrctl images' instead)")
	}
}
