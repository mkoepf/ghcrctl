package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/gh"
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
			filter, err := BuildVersionFilter(
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

// TestListVersionsCommandStructure verifies the list versions command is properly set up
func TestListVersionsCommandStructure(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	versionsCmd, _, err := cmd.Find([]string{"list", "versions"})
	if err != nil {
		t.Fatalf("Failed to find list versions command: %v", err)
	}

	if versionsCmd.Use != "versions <owner/package>" {
		t.Errorf("Expected Use 'versions <owner/package>', got '%s'", versionsCmd.Use)
	}

	if versionsCmd.Short == "" {
		t.Error("Short description should not be empty")
	}
}

// TestListVersionsCommandArguments verifies argument validation
func TestListVersionsCommandArguments(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectUsage bool
	}{
		{
			name:        "missing image argument",
			args:        []string{"list", "versions"},
			expectUsage: true,
		},
		{
			name:        "too many arguments",
			args:        []string{"list", "versions", "myimage", "extra"},
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
		"older-than-days",
		"newer-than-days",
		"version",
		"digest",
	}

	for _, flagName := range requiredFlags {
		flag := versionsCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("list versions command should have --%s flag", flagName)
		}
	}
}

// TestListVersionsCommandNoTreeFlag verifies --tree flag was removed
// (use 'ghcrctl list images' for tree view instead)
func TestListVersionsCommandNoTreeFlag(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	versionsCmd, _, _ := cmd.Find([]string{"list", "versions"})

	flag := versionsCmd.Flags().Lookup("tree")
	if flag != nil {
		t.Error("list versions command should NOT have --tree flag (use 'ghcrctl list images' instead)")
	}
}

// TestOutputListVersionsTableQuietMode verifies quiet mode suppresses informational output
func TestOutputListVersionsTableQuietMode(t *testing.T) {
	t.Parallel()
	versions := []gh.PackageVersionInfo{
		{ID: 123, Name: "sha256:abc123", Tags: []string{"v1.0.0"}, CreatedAt: "2025-01-01"},
	}

	// Normal mode should include header and summary
	var normalBuf bytes.Buffer
	err := OutputVersionsTable(&normalBuf, versions, "testpkg", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	normalOutput := normalBuf.String()
	if !strings.Contains(normalOutput, "Versions for testpkg") {
		t.Error("normal mode should include 'Versions for' header")
	}
	if !strings.Contains(normalOutput, "Total:") {
		t.Error("normal mode should include 'Total:' summary")
	}

	// Quiet mode should NOT include header or summary
	var quietBuf bytes.Buffer
	err = OutputVersionsTable(&quietBuf, versions, "testpkg", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	quietOutput := quietBuf.String()
	if strings.Contains(quietOutput, "Versions for testpkg") {
		t.Error("quiet mode should NOT include 'Versions for' header")
	}
	if strings.Contains(quietOutput, "Total:") {
		t.Error("quiet mode should NOT include 'Total:' summary")
	}
	// But should still have data
	if !strings.Contains(quietOutput, "123") {
		t.Error("quiet mode should still include version ID")
	}
}
