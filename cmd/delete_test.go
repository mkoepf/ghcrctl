package cmd

import (
	"testing"

	"github.com/mhk/ghcrctl/internal/discovery"
	"github.com/mhk/ghcrctl/internal/gh"
	"github.com/mhk/ghcrctl/internal/oras"
)

// TestDeleteCommandStructure verifies the delete command is properly set up
func TestDeleteCommandStructure(t *testing.T) {
	cmd := rootCmd
	deleteCmd, _, err := cmd.Find([]string{"delete"})
	if err != nil {
		t.Fatalf("Failed to find delete command: %v", err)
	}

	if deleteCmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	// Check for subcommands
	subcommands := deleteCmd.Commands()
	if len(subcommands) < 2 {
		t.Errorf("Expected at least 2 subcommands (version, graph), got %d", len(subcommands))
	}
}

// TestDeleteVersionCommandStructure verifies the delete version subcommand
func TestDeleteVersionCommandStructure(t *testing.T) {
	cmd := rootCmd
	deleteVersionCmd, _, err := cmd.Find([]string{"delete", "version"})
	if err != nil {
		t.Fatalf("Failed to find delete version command: %v", err)
	}

	if deleteVersionCmd.Use != "version <image> [version-id]" {
		t.Errorf("Expected Use 'version <image> [version-id]', got '%s'", deleteVersionCmd.Use)
	}

	if deleteVersionCmd.Short == "" {
		t.Error("Short description should not be empty")
	}
}

// TestDeleteVersionCommandArguments verifies argument validation
func TestDeleteVersionCommandArguments(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectUsage bool
	}{
		{
			name:        "missing all arguments",
			args:        []string{"delete", "version"},
			expectUsage: true,
		},
		{
			name:        "missing version-id argument",
			args:        []string{"delete", "version", "myimage"},
			expectUsage: true,
		},
		{
			name:        "too many arguments",
			args:        []string{"delete", "version", "myimage", "12345", "extra"},
			expectUsage: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			// Should fail with usage error
			if err == nil {
				t.Error("Expected error but got none")
			}

			// Reset args
			rootCmd.SetArgs([]string{})
		})
	}
}

// TestDeleteVersionCommandHasFlags verifies required flags exist
func TestDeleteVersionCommandHasFlags(t *testing.T) {
	cmd := rootCmd
	deleteVersionCmd, _, err := cmd.Find([]string{"delete", "version"})
	if err != nil {
		t.Fatalf("Failed to find delete version command: %v", err)
	}

	requiredFlags := []string{
		"force",
		"dry-run",
		"digest",
		"tag-pattern",
		"tagged",
		"untagged",
		"older-than",
		"newer-than",
		"older-than-days",
		"newer-than-days",
	}

	for _, flagName := range requiredFlags {
		flag := deleteVersionCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("delete version command should have --%s flag", flagName)
		}
	}
}

// TestDeleteGraphCommandStructure verifies the delete graph subcommand
func TestDeleteGraphCommandStructure(t *testing.T) {
	cmd := rootCmd
	deleteGraphCmd, _, err := cmd.Find([]string{"delete", "graph"})
	if err != nil {
		t.Fatalf("Failed to find delete graph command: %v", err)
	}

	if deleteGraphCmd.Use != "graph <image> <tag>" {
		t.Errorf("Expected Use 'graph <image> <tag>', got '%s'", deleteGraphCmd.Use)
	}

	if deleteGraphCmd.Short == "" {
		t.Error("Short description should not be empty")
	}
}

// TestDeleteGraphCommandArguments verifies argument validation
func TestDeleteGraphCommandArguments(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectUsage bool
	}{
		{
			name:        "missing all arguments",
			args:        []string{"delete", "graph"},
			expectUsage: true,
		},
		{
			name:        "missing tag argument",
			args:        []string{"delete", "graph", "myimage"},
			expectUsage: true,
		},
		{
			name:        "too many arguments",
			args:        []string{"delete", "graph", "myimage", "v1.0.0", "extra"},
			expectUsage: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			// Should fail with usage error
			if err == nil {
				t.Error("Expected error but got none")
			}

			// Reset args
			rootCmd.SetArgs([]string{})
		})
	}
}

// TestDeleteGraphCommandHasFlags verifies required flags exist
func TestDeleteGraphCommandHasFlags(t *testing.T) {
	cmd := rootCmd
	deleteGraphCmd, _, err := cmd.Find([]string{"delete", "graph"})
	if err != nil {
		t.Fatalf("Failed to find delete graph command: %v", err)
	}

	// Check for --force flag
	forceFlag := deleteGraphCmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Error("Expected --force flag to exist")
	}

	// Check for --dry-run flag
	dryRunFlag := deleteGraphCmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Error("Expected --dry-run flag to exist")
	}

	// Check for --digest flag
	digestFlag := deleteGraphCmd.Flags().Lookup("digest")
	if digestFlag == nil {
		t.Error("Expected --digest flag to exist")
	}

	// Check for --version flag
	versionFlag := deleteGraphCmd.Flags().Lookup("version")
	if versionFlag == nil {
		t.Error("Expected --version flag to exist")
	}
}

// TestDeleteGraphCommandFlagExclusivity verifies mutually exclusive flags
func TestDeleteGraphCommandFlagExclusivity(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		expectErr bool
	}{
		{
			name:      "tag and digest flags both set",
			args:      []string{"delete", "graph", "myimage", "v1.0.0", "--digest", "sha256:abc"},
			expectErr: true,
		},
		{
			name:      "tag and version flags both set",
			args:      []string{"delete", "graph", "myimage", "v1.0.0", "--version", "12345"},
			expectErr: true,
		},
		{
			name:      "digest and version flags both set",
			args:      []string{"delete", "graph", "myimage", "--digest", "sha256:abc", "--version", "12345"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			if tt.expectErr && err == nil {
				t.Error("Expected error for mutually exclusive flags, got none")
			}

			// Reset args
			rootCmd.SetArgs([]string{})
		})
	}
}

// TestCollectVersionIDsExcludesSharedChildren verifies that collectVersionIDs
// excludes children that are shared with other graphs (RefCount > 1)
func TestCollectVersionIDsExcludesSharedChildren(t *testing.T) {
	// Test that shared children (RefCount > 1) are NOT included in deletion
	// Only the current graph's root and exclusive children should be deleted

	graph := &discovery.VersionGraph{
		RootVersion: gh.PackageVersionInfo{ID: 100, Name: "sha256:root"},
		Type:        "index",
		Children: []discovery.VersionChild{
			// Exclusive platform (only in this graph)
			{Version: gh.PackageVersionInfo{ID: 101, Name: "sha256:exclusive-platform"}, Type: oras.ArtifactType{ManifestType: "manifest", Role: "platform", Platform: "linux/amd64"}, RefCount: 1},
			// Shared platform (in 2 graphs) - should be EXCLUDED
			{Version: gh.PackageVersionInfo{ID: 102, Name: "sha256:shared-platform"}, Type: oras.ArtifactType{ManifestType: "manifest", Role: "platform", Platform: "linux/arm64"}, RefCount: 2},
			// Exclusive attestation
			{Version: gh.PackageVersionInfo{ID: 103, Name: "sha256:exclusive-sbom"}, Type: oras.ArtifactType{ManifestType: "manifest", Role: "sbom"}, RefCount: 1},
			// Shared attestation - should be EXCLUDED
			{Version: gh.PackageVersionInfo{ID: 104, Name: "sha256:shared-prov"}, Type: oras.ArtifactType{ManifestType: "manifest", Role: "provenance"}, RefCount: 3},
		},
	}

	ids := collectVersionIDs(graph)

	// Should include: root (100), exclusive platform (101), exclusive sbom (103)
	// Should exclude: shared platform (102), shared prov (104)
	expectedIDs := map[int64]bool{100: true, 101: true, 103: true}
	excludedIDs := map[int64]bool{102: true, 104: true}

	for _, id := range ids {
		if excludedIDs[id] {
			t.Errorf("collectVersionIDs should NOT include shared child ID %d", id)
		}
		delete(expectedIDs, id)
	}

	if len(expectedIDs) > 0 {
		for id := range expectedIDs {
			t.Errorf("collectVersionIDs should include exclusive/root ID %d", id)
		}
	}
}

// TestBuildDeleteFilter verifies that the filter is built correctly from flags
func TestBuildDeleteFilter(t *testing.T) {
	tests := []struct {
		name        string
		setup       func()
		wantErr     bool
		errContains string
	}{
		{
			name: "no filters",
			setup: func() {
				deleteTagPattern = ""
				deleteOnlyTagged = false
				deleteOnlyUntagged = false
				deleteOlderThan = ""
				deleteNewerThan = ""
				deleteOlderThanDays = 0
				deleteNewerThanDays = 0
			},
			wantErr: false,
		},
		{
			name: "conflicting tagged/untagged flags",
			setup: func() {
				deleteOnlyTagged = true
				deleteOnlyUntagged = true
			},
			wantErr:     true,
			errContains: "cannot use --tagged and --untagged together",
		},
		{
			name: "invalid older-than date",
			setup: func() {
				deleteOnlyTagged = false
				deleteOnlyUntagged = false
				deleteOlderThan = "invalid-date"
			},
			wantErr:     true,
			errContains: "invalid --older-than date format",
		},
		{
			name: "valid older-than date RFC3339",
			setup: func() {
				deleteOlderThan = "2025-01-01T00:00:00Z"
				deleteNewerThan = ""
			},
			wantErr: false,
		},
		{
			name: "valid older-than date (date only)",
			setup: func() {
				deleteOlderThan = "2025-01-01"
				deleteNewerThan = ""
			},
			wantErr: false,
		},
		{
			name: "valid newer-than date (date only)",
			setup: func() {
				deleteOlderThan = ""
				deleteNewerThan = "2025-11-01"
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			filter, err := buildDeleteFilter()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errContains != "" && !containsStr(err.Error(), tt.errContains) {
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

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstr(s, substr)))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestDeleteVersionBulkModeArgsValidation tests that bulk mode accepts only image name
func TestDeleteVersionBulkModeArgsValidation(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		setupFlag func()
		expectErr bool
	}{
		{
			name: "bulk mode with untagged flag - correct args",
			args: []string{"delete", "version", "myimage"},
			setupFlag: func() {
				deleteOnlyUntagged = true
			},
			expectErr: false,
		},
		{
			name: "bulk mode with untagged flag - too many args",
			args: []string{"delete", "version", "myimage", "12345"},
			setupFlag: func() {
				deleteOnlyUntagged = true
			},
			expectErr: true,
		},
		{
			name: "bulk mode with tag pattern - correct args",
			args: []string{"delete", "version", "myimage"},
			setupFlag: func() {
				deleteOnlyUntagged = false
				deleteTagPattern = ".*-rc.*"
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags
			deleteOnlyUntagged = false
			deleteTagPattern = ""

			// Setup specific flag
			if tt.setupFlag != nil {
				tt.setupFlag()
			}

			// Test args validation
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			// We expect configuration errors since we're not providing real tokens/config
			// But we should not get args validation errors if expectErr is false
			if !tt.expectErr && err != nil {
				// Check if error is about args validation (not config/auth errors)
				errStr := err.Error()
				if containsStr(errStr, "accepts") || containsStr(errStr, "arg") {
					t.Errorf("Unexpected args validation error: %v", err)
				}
			}

			// Reset
			rootCmd.SetArgs([]string{})
			deleteOnlyUntagged = false
			deleteTagPattern = ""
		})
	}
}

// TestFormatTagsForDisplay tests the tags formatting function
func TestFormatTagsForDisplay(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want string
	}{
		{
			name: "no tags should show []",
			tags: []string{},
			want: "[]",
		},
		{
			name: "nil tags should show []",
			tags: nil,
			want: "[]",
		},
		{
			name: "single tag",
			tags: []string{"v1.0"},
			want: "v1.0",
		},
		{
			name: "multiple tags",
			tags: []string{"v1.0", "latest"},
			want: "v1.0, latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTagsForDisplay(tt.tags)
			if got != tt.want {
				t.Errorf("FormatTagsForDisplay() = %q, want %q", got, tt.want)
			}
		})
	}
}
