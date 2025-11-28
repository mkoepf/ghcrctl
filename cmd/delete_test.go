package cmd

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/discovery"
	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/mkoepf/ghcrctl/internal/oras"
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
			{Version: gh.PackageVersionInfo{ID: 101, Name: "sha256:exclusive-platform"}, Type: oras.ArtifactType{Role: "platform", Platform: "linux/amd64"}, RefCount: 1},
			// Shared platform (in 2 graphs) - should be EXCLUDED
			{Version: gh.PackageVersionInfo{ID: 102, Name: "sha256:shared-platform"}, Type: oras.ArtifactType{Role: "platform", Platform: "linux/arm64"}, RefCount: 2},
			// Exclusive attestation
			{Version: gh.PackageVersionInfo{ID: 103, Name: "sha256:exclusive-sbom"}, Type: oras.ArtifactType{Role: "sbom"}, RefCount: 1},
			// Shared attestation - should be EXCLUDED
			{Version: gh.PackageVersionInfo{ID: 104, Name: "sha256:shared-prov"}, Type: oras.ArtifactType{Role: "provenance"}, RefCount: 3},
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

// TestDisplayGraphSummary tests the human-readable graph summary output
func TestDisplayGraphSummary(t *testing.T) {
	tests := []struct {
		name           string
		graph          *discovery.VersionGraph
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "simple root with no children",
			graph: &discovery.VersionGraph{
				RootVersion: gh.PackageVersionInfo{
					ID:   100,
					Name: "sha256:rootdigest123",
					Tags: []string{"v1.0.0"},
				},
				Type:     "standalone",
				Children: []discovery.VersionChild{},
			},
			wantContains: []string{
				"Root (Image): sha256:rootdigest123",
				"Tags: [v1.0.0]",
				"Version ID: 100",
			},
			wantNotContain: []string{
				"Platforms to delete",
				"Attestations to delete",
				"Shared artifacts",
			},
		},
		{
			name: "index with exclusive platforms",
			graph: &discovery.VersionGraph{
				RootVersion: gh.PackageVersionInfo{
					ID:   100,
					Name: "sha256:indexdigest",
					Tags: []string{"latest"},
				},
				Type: "index",
				Children: []discovery.VersionChild{
					{
						Version:  gh.PackageVersionInfo{ID: 101, Name: "sha256:amd64"},
						Type:     oras.ArtifactType{Role: "platform", Platform: "linux/amd64"},
						RefCount: 1,
					},
					{
						Version:  gh.PackageVersionInfo{ID: 102, Name: "sha256:arm64"},
						Type:     oras.ArtifactType{Role: "platform", Platform: "linux/arm64"},
						RefCount: 1,
					},
				},
			},
			wantContains: []string{
				"Root (Image): sha256:indexdigest",
				"Platforms to delete (2)",
				"linux/amd64 (version 101)",
				"linux/arm64 (version 102)",
			},
			wantNotContain: []string{
				"Shared artifacts",
			},
		},
		{
			name: "manifest with exclusive attestations",
			graph: &discovery.VersionGraph{
				RootVersion: gh.PackageVersionInfo{
					ID:   200,
					Name: "sha256:manifestdigest",
					Tags: []string{},
				},
				Type: "manifest",
				Children: []discovery.VersionChild{
					{
						Version:  gh.PackageVersionInfo{ID: 201, Name: "sha256:sbomdigest"},
						Type:     oras.ArtifactType{Role: "sbom"},
						RefCount: 1,
					},
					{
						Version:  gh.PackageVersionInfo{ID: 202, Name: "sha256:provdigest"},
						Type:     oras.ArtifactType{Role: "provenance"},
						RefCount: 1,
					},
				},
			},
			wantContains: []string{
				"Root (Image): sha256:manifestdigest",
				"Attestations to delete (2)",
				"sbom (version 201)",
				"provenance (version 202)",
			},
			wantNotContain: []string{
				"Tags:",
				"Shared artifacts",
				"Platforms to delete",
			},
		},
		{
			name: "graph with shared platforms (preserved)",
			graph: &discovery.VersionGraph{
				RootVersion: gh.PackageVersionInfo{
					ID:   300,
					Name: "sha256:rootwithshared",
					Tags: []string{"v2.0"},
				},
				Type: "index",
				Children: []discovery.VersionChild{
					{
						Version:  gh.PackageVersionInfo{ID: 301, Name: "sha256:exclusive"},
						Type:     oras.ArtifactType{Role: "platform", Platform: "linux/amd64"},
						RefCount: 1,
					},
					{
						Version:  gh.PackageVersionInfo{ID: 302, Name: "sha256:shared"},
						Type:     oras.ArtifactType{Role: "platform", Platform: "linux/arm64"},
						RefCount: 3,
					},
				},
			},
			wantContains: []string{
				"Platforms to delete (1)",
				"linux/amd64 (version 301)",
				"Shared artifacts (preserved, used by other graphs)",
				"linux/arm64 (version 302, shared by 3 graphs)",
			},
		},
		{
			name: "graph with shared attestations",
			graph: &discovery.VersionGraph{
				RootVersion: gh.PackageVersionInfo{
					ID:   400,
					Name: "sha256:rootwithsharedatt",
					Tags: []string{},
				},
				Type: "manifest",
				Children: []discovery.VersionChild{
					{
						Version:  gh.PackageVersionInfo{ID: 401, Name: "sha256:exclusivesbom"},
						Type:     oras.ArtifactType{Role: "sbom"},
						RefCount: 1,
					},
					{
						Version:  gh.PackageVersionInfo{ID: 402, Name: "sha256:sharedprov"},
						Type:     oras.ArtifactType{Role: "provenance"},
						RefCount: 2,
					},
				},
			},
			wantContains: []string{
				"Attestations to delete (1)",
				"sbom (version 401)",
				"Shared artifacts (preserved, used by other graphs)",
				"provenance (version 402, shared by 2 graphs)",
			},
		},
		{
			name: "complex graph with mix of exclusive and shared",
			graph: &discovery.VersionGraph{
				RootVersion: gh.PackageVersionInfo{
					ID:   500,
					Name: "sha256:complexroot",
					Tags: []string{"v3.0", "stable"},
				},
				Type: "index",
				Children: []discovery.VersionChild{
					// Exclusive platform
					{
						Version:  gh.PackageVersionInfo{ID: 501, Name: "sha256:excplatform"},
						Type:     oras.ArtifactType{Role: "platform", Platform: "linux/amd64"},
						RefCount: 1,
					},
					// Shared platform
					{
						Version:  gh.PackageVersionInfo{ID: 502, Name: "sha256:sharedplatform"},
						Type:     oras.ArtifactType{Role: "platform", Platform: "linux/arm64"},
						RefCount: 2,
					},
					// Exclusive attestation
					{
						Version:  gh.PackageVersionInfo{ID: 503, Name: "sha256:excsbom"},
						Type:     oras.ArtifactType{Role: "sbom"},
						RefCount: 1,
					},
					// Shared attestation
					{
						Version:  gh.PackageVersionInfo{ID: 504, Name: "sha256:sharedprov"},
						Type:     oras.ArtifactType{Role: "provenance"},
						RefCount: 4,
					},
				},
			},
			wantContains: []string{
				"Root (Image): sha256:complexroot",
				"Tags: [v3.0 stable]",
				"Platforms to delete (1)",
				"linux/amd64 (version 501)",
				"Attestations to delete (1)",
				"sbom (version 503)",
				"Shared artifacts (preserved, used by other graphs)",
				"linux/arm64 (version 502, shared by 2 graphs)",
				"provenance (version 504, shared by 4 graphs)",
			},
		},
		{
			name: "root without version ID",
			graph: &discovery.VersionGraph{
				RootVersion: gh.PackageVersionInfo{
					ID:   0, // Not yet resolved
					Name: "sha256:noidroot",
					Tags: []string{"test"},
				},
				Type:     "standalone",
				Children: []discovery.VersionChild{},
			},
			wantContains: []string{
				"Root (Image): sha256:noidroot",
				"Tags: [test]",
			},
			wantNotContain: []string{
				"Version ID:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			displayGraphSummary(&buf, tt.graph)
			output := buf.String()

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("Expected output to contain %q\nGot:\n%s", want, output)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(output, notWant) {
					t.Errorf("Expected output NOT to contain %q\nGot:\n%s", notWant, output)
				}
			}
		})
	}
}

// TestCollectVersionIDsDeletionOrder verifies that versions are collected in correct deletion order
// (children before root to prevent orphaning)
func TestCollectVersionIDsDeletionOrder(t *testing.T) {
	graph := &discovery.VersionGraph{
		RootVersion: gh.PackageVersionInfo{ID: 100, Name: "sha256:root"},
		Type:        "index",
		Children: []discovery.VersionChild{
			// Platform
			{Version: gh.PackageVersionInfo{ID: 101, Name: "sha256:platform"}, Type: oras.ArtifactType{Role: "platform", Platform: "linux/amd64"}, RefCount: 1},
			// Attestation
			{Version: gh.PackageVersionInfo{ID: 102, Name: "sha256:sbom"}, Type: oras.ArtifactType{Role: "sbom"}, RefCount: 1},
		},
	}

	ids := collectVersionIDs(graph)

	// Root should always be last
	if len(ids) == 0 {
		t.Fatal("Expected at least one ID")
	}

	lastID := ids[len(ids)-1]
	if lastID != 100 {
		t.Errorf("Root ID (100) should be last in deletion order, got %d as last", lastID)
	}

	// Attestations should come before platforms (though both before root)
	sbomIdx := -1
	platformIdx := -1
	for i, id := range ids {
		if id == 102 {
			sbomIdx = i
		}
		if id == 101 {
			platformIdx = i
		}
	}

	if sbomIdx == -1 || platformIdx == -1 {
		t.Error("Both sbom and platform IDs should be present")
	}

	// Attestations (sbom) come before platforms in the implementation
	if sbomIdx > platformIdx {
		t.Errorf("Attestations should come before platforms in deletion order. sbomIdx=%d, platformIdx=%d", sbomIdx, platformIdx)
	}
}

// TestCollectVersionIDsEmptyGraph tests handling of edge cases
func TestCollectVersionIDsEmptyGraph(t *testing.T) {
	tests := []struct {
		name     string
		graph    *discovery.VersionGraph
		wantLen  int
		wantRoot bool
	}{
		{
			name: "only root",
			graph: &discovery.VersionGraph{
				RootVersion: gh.PackageVersionInfo{ID: 100, Name: "sha256:onlyroot"},
				Children:    []discovery.VersionChild{},
			},
			wantLen:  1,
			wantRoot: true,
		},
		{
			name: "root with zero ID",
			graph: &discovery.VersionGraph{
				RootVersion: gh.PackageVersionInfo{ID: 0, Name: "sha256:noid"},
				Children:    []discovery.VersionChild{},
			},
			wantLen:  0, // Zero ID means not resolved, should not be included
			wantRoot: false,
		},
		{
			name: "children with zero ID",
			graph: &discovery.VersionGraph{
				RootVersion: gh.PackageVersionInfo{ID: 100, Name: "sha256:root"},
				Children: []discovery.VersionChild{
					{Version: gh.PackageVersionInfo{ID: 0, Name: "sha256:noid"}, Type: oras.ArtifactType{Role: "platform"}, RefCount: 1},
					{Version: gh.PackageVersionInfo{ID: 101, Name: "sha256:hasid"}, Type: oras.ArtifactType{Role: "platform"}, RefCount: 1},
				},
			},
			wantLen:  2, // root + one valid child
			wantRoot: true,
		},
		{
			name: "all children shared",
			graph: &discovery.VersionGraph{
				RootVersion: gh.PackageVersionInfo{ID: 100, Name: "sha256:root"},
				Children: []discovery.VersionChild{
					{Version: gh.PackageVersionInfo{ID: 101, Name: "sha256:shared1"}, Type: oras.ArtifactType{Role: "platform"}, RefCount: 2},
					{Version: gh.PackageVersionInfo{ID: 102, Name: "sha256:shared2"}, Type: oras.ArtifactType{Role: "sbom"}, RefCount: 3},
				},
			},
			wantLen:  1, // only root, all children are shared
			wantRoot: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids := collectVersionIDs(tt.graph)

			if len(ids) != tt.wantLen {
				t.Errorf("Expected %d IDs, got %d: %v", tt.wantLen, len(ids), ids)
			}

			hasRoot := false
			for _, id := range ids {
				if id == tt.graph.RootVersion.ID && id != 0 {
					hasRoot = true
				}
			}

			if tt.wantRoot && !hasRoot {
				t.Error("Expected root ID to be included")
			}
			if !tt.wantRoot && hasRoot {
				t.Error("Expected root ID NOT to be included")
			}
		})
	}
}

// TestCollectVersionIDsRefCountBoundary tests the RefCount boundary condition
func TestCollectVersionIDsRefCountBoundary(t *testing.T) {
	tests := []struct {
		name        string
		refCount    int
		shouldIncl  bool
		description string
	}{
		{name: "RefCount 0", refCount: 0, shouldIncl: true, description: "unset/unknown should be included"},
		{name: "RefCount 1", refCount: 1, shouldIncl: true, description: "exclusive child should be included"},
		{name: "RefCount 2", refCount: 2, shouldIncl: false, description: "shared child should be excluded"},
		{name: "RefCount 10", refCount: 10, shouldIncl: false, description: "highly shared child should be excluded"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph := &discovery.VersionGraph{
				RootVersion: gh.PackageVersionInfo{ID: 100, Name: "sha256:root"},
				Children: []discovery.VersionChild{
					{
						Version:  gh.PackageVersionInfo{ID: 101, Name: "sha256:child"},
						Type:     oras.ArtifactType{Role: "platform", Platform: "linux/amd64"},
						RefCount: tt.refCount,
					},
				},
			}

			ids := collectVersionIDs(graph)

			childIncluded := false
			for _, id := range ids {
				if id == 101 {
					childIncluded = true
				}
			}

			if tt.shouldIncl && !childIncluded {
				t.Errorf("%s: expected child to be included (RefCount=%d)", tt.description, tt.refCount)
			}
			if !tt.shouldIncl && childIncluded {
				t.Errorf("%s: expected child to be excluded (RefCount=%d)", tt.description, tt.refCount)
			}
		})
	}
}

// TestCountVersionInGraphs tests the graph membership counting logic
func TestCountVersionInGraphs(t *testing.T) {
	// Create test graphs
	graphs := []discovery.VersionGraph{
		{
			RootVersion: gh.PackageVersionInfo{ID: 100, Name: "sha256:graph1root"},
			Children: []discovery.VersionChild{
				{Version: gh.PackageVersionInfo{ID: 101, Name: "sha256:child1"}},
				{Version: gh.PackageVersionInfo{ID: 102, Name: "sha256:child2"}},
			},
		},
		{
			RootVersion: gh.PackageVersionInfo{ID: 200, Name: "sha256:graph2root"},
			Children: []discovery.VersionChild{
				{Version: gh.PackageVersionInfo{ID: 102, Name: "sha256:child2"}}, // shared with graph1
				{Version: gh.PackageVersionInfo{ID: 201, Name: "sha256:child3"}},
			},
		},
		{
			RootVersion: gh.PackageVersionInfo{ID: 300, Name: "sha256:graph3root"},
			Children:    []discovery.VersionChild{}, // standalone with no children
		},
	}

	tests := []struct {
		name      string
		versionID int64
		wantCount int
	}{
		{
			name:      "version is root of one graph",
			versionID: 100,
			wantCount: 1,
		},
		{
			name:      "version is exclusive child (in one graph)",
			versionID: 101,
			wantCount: 1,
		},
		{
			name:      "version is shared child (in two graphs)",
			versionID: 102,
			wantCount: 2,
		},
		{
			name:      "version not in any graph",
			versionID: 999,
			wantCount: 0,
		},
		{
			name:      "standalone root with no children",
			versionID: 300,
			wantCount: 1,
		},
		{
			name:      "version ID 0 (unresolved) not in any graph",
			versionID: 0,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countVersionInGraphs(graphs, tt.versionID)
			if got != tt.wantCount {
				t.Errorf("countVersionInGraphs() = %d, want %d", got, tt.wantCount)
			}
		})
	}
}

// TestCountVersionInGraphsEmptyGraphs tests edge case of empty graphs slice
func TestCountVersionInGraphsEmptyGraphs(t *testing.T) {
	count := countVersionInGraphs([]discovery.VersionGraph{}, 100)
	if count != 0 {
		t.Errorf("Expected 0 for empty graphs, got %d", count)
	}

	count = countVersionInGraphs(nil, 100)
	if count != 0 {
		t.Errorf("Expected 0 for nil graphs, got %d", count)
	}
}

// TestFindGraphByDigest tests the graph lookup by digest
func TestFindGraphByDigest(t *testing.T) {
	graphs := []discovery.VersionGraph{
		{
			RootVersion: gh.PackageVersionInfo{ID: 100, Name: "sha256:digest1"},
			Children: []discovery.VersionChild{
				{Version: gh.PackageVersionInfo{ID: 101}, RefCount: 1},
			},
		},
		{
			RootVersion: gh.PackageVersionInfo{ID: 200, Name: "sha256:digest2"},
			Children: []discovery.VersionChild{
				{Version: gh.PackageVersionInfo{ID: 201}, RefCount: 3},
			},
		},
	}

	tests := []struct {
		name           string
		rootDigest     string
		wantRootID     int64
		wantFound      bool
		checkFallback  bool
		fallbackChilds int
	}{
		{
			name:       "find existing graph by digest",
			rootDigest: "sha256:digest1",
			wantRootID: 100,
			wantFound:  true,
		},
		{
			name:       "find second graph by digest",
			rootDigest: "sha256:digest2",
			wantRootID: 200,
			wantFound:  true,
		},
		{
			name:           "digest not found - uses fallback with conservative RefCount",
			rootDigest:     "sha256:notfound",
			wantFound:      false,
			checkFallback:  true,
			fallbackChilds: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fallback *discovery.VersionGraph
			if tt.checkFallback {
				fallback = &discovery.VersionGraph{
					RootVersion: gh.PackageVersionInfo{ID: 999, Name: "sha256:fallback"},
					Children: []discovery.VersionChild{
						{Version: gh.PackageVersionInfo{ID: 1001}, RefCount: 0},
						{Version: gh.PackageVersionInfo{ID: 1002}, RefCount: 1},
					},
				}
			}

			got, err := findGraphByDigest(graphs, tt.rootDigest, fallback)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.wantFound {
				if got == nil {
					t.Fatal("Expected to find graph, got nil")
				}
				if got.RootVersion.ID != tt.wantRootID {
					t.Errorf("Got root ID %d, want %d", got.RootVersion.ID, tt.wantRootID)
				}
			} else if tt.checkFallback {
				if got == nil {
					t.Fatal("Expected fallback graph, got nil")
				}
				if got.RootVersion.ID != 999 {
					t.Errorf("Expected fallback root ID 999, got %d", got.RootVersion.ID)
				}
				// Check that conservative RefCount was applied
				for i, child := range got.Children {
					if child.RefCount != 2 {
						t.Errorf("Child %d should have conservative RefCount 2, got %d", i, child.RefCount)
					}
				}
			}
		})
	}
}

// TestFindGraphByDigestNilFallback tests handling of nil fallback
func TestFindGraphByDigestNilFallback(t *testing.T) {
	graphs := []discovery.VersionGraph{}

	got, err := findGraphByDigest(graphs, "sha256:notfound", nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("Expected nil when digest not found and fallback is nil, got %+v", got)
	}
}

// TestFindGraphByDigestEmptyGraphs tests with empty graphs slice
func TestFindGraphByDigestEmptyGraphs(t *testing.T) {
	fallback := &discovery.VersionGraph{
		RootVersion: gh.PackageVersionInfo{ID: 100, Name: "sha256:fallback"},
		Children: []discovery.VersionChild{
			{Version: gh.PackageVersionInfo{ID: 101}, RefCount: 0},
		},
	}

	got, err := findGraphByDigest([]discovery.VersionGraph{}, "sha256:any", fallback)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("Expected fallback to be returned")
	}
	if got.Children[0].RefCount != 2 {
		t.Errorf("Expected conservative RefCount 2, got %d", got.Children[0].RefCount)
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

// =============================================================================
// Mock implementations for testing
// =============================================================================

// mockPackageDeleter implements gh.PackageDeleter for testing
type mockPackageDeleter struct {
	deletedVersions []int64
	deleteErrors    map[int64]error // Map of versionID -> error to return
	callCount       int
}

func newMockPackageDeleter() *mockPackageDeleter {
	return &mockPackageDeleter{
		deletedVersions: []int64{},
		deleteErrors:    make(map[int64]error),
	}
}

func (m *mockPackageDeleter) DeletePackageVersion(ctx context.Context, owner, ownerType, packageName string, versionID int64) error {
	m.callCount++
	if err, ok := m.deleteErrors[versionID]; ok {
		return err
	}
	m.deletedVersions = append(m.deletedVersions, versionID)
	return nil
}

// =============================================================================
// Tests for deleteGraphWithDeleter
// =============================================================================

func TestDeleteGraphWithDeleter(t *testing.T) {
	tests := []struct {
		name           string
		versionIDs     []int64
		deleteErrors   map[int64]error
		wantDeleted    []int64
		wantErr        bool
		wantErrContain string
		wantOutput     []string
	}{
		{
			name:        "successful deletion of all versions",
			versionIDs:  []int64{201, 202, 100}, // attestations, platform, root
			wantDeleted: []int64{201, 202, 100},
			wantErr:     false,
			wantOutput: []string{
				"Deleting version 1/3 (ID: 201)",
				"Deleting version 2/3 (ID: 202)",
				"Deleting version 3/3 (ID: 100)",
			},
		},
		{
			name:        "single version deletion",
			versionIDs:  []int64{12345},
			wantDeleted: []int64{12345},
			wantErr:     false,
			wantOutput:  []string{"Deleting version 1/1 (ID: 12345)"},
		},
		{
			name:        "empty version list succeeds without action",
			versionIDs:  []int64{},
			wantDeleted: []int64{},
			wantErr:     false,
		},
		{
			name:           "stops on first error",
			versionIDs:     []int64{201, 202, 100},
			deleteErrors:   map[int64]error{202: fmt.Errorf("permission denied")},
			wantDeleted:    []int64{201}, // only first one succeeds before error
			wantErr:        true,
			wantErrContain: "failed to delete version 202",
			wantOutput:     []string{"Deleting version 1/3", "Deleting version 2/3"},
		},
		{
			name:           "error on first version",
			versionIDs:     []int64{100, 200},
			deleteErrors:   map[int64]error{100: fmt.Errorf("not found")},
			wantDeleted:    []int64{},
			wantErr:        true,
			wantErrContain: "failed to delete version 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockPackageDeleter()
			mock.deleteErrors = tt.deleteErrors

			var buf strings.Builder
			ctx := context.Background()

			err := deleteGraphWithDeleter(ctx, mock, "owner", "user", "image", tt.versionIDs, &buf)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErrContain != "" && err != nil {
				if !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErrContain)
				}
			}

			// Check deleted versions
			if len(mock.deletedVersions) != len(tt.wantDeleted) {
				t.Errorf("deleted %d versions, want %d", len(mock.deletedVersions), len(tt.wantDeleted))
			}
			for i, id := range tt.wantDeleted {
				if i < len(mock.deletedVersions) && mock.deletedVersions[i] != id {
					t.Errorf("deletedVersions[%d] = %d, want %d", i, mock.deletedVersions[i], id)
				}
			}

			// Check output
			output := buf.String()
			for _, want := range tt.wantOutput {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\nGot:\n%s", want, output)
				}
			}
		})
	}
}

// =============================================================================
// Tests for executeSingleDelete
// =============================================================================

func TestExecuteSingleDelete(t *testing.T) {
	tests := []struct {
		name            string
		params          deleteVersionParams
		confirmResponse bool
		confirmErr      error
		deleteErr       error
		wantDeleted     bool
		wantErr         bool
		wantOutput      []string
		wantNotOutput   []string
	}{
		{
			name: "successful deletion with force",
			params: deleteVersionParams{
				owner:      "testowner",
				ownerType:  "user",
				imageName:  "testimage",
				versionID:  12345,
				tags:       []string{"v1.0", "latest"},
				graphCount: 0,
				force:      true,
				dryRun:     false,
			},
			wantDeleted: true,
			wantErr:     false,
			wantOutput: []string{
				"Preparing to delete package version",
				"Image:      testimage",
				"Owner:      testowner (user)",
				"Version ID: 12345",
				"Tags:       v1.0, latest",
				"Successfully deleted version 12345",
			},
		},
		{
			name: "dry run does not delete",
			params: deleteVersionParams{
				owner:     "testowner",
				ownerType: "org",
				imageName: "testimage",
				versionID: 67890,
				tags:      []string{},
				force:     false,
				dryRun:    true,
			},
			wantDeleted: false,
			wantErr:     false,
			wantOutput: []string{
				"DRY RUN: No changes made",
				"Tags:       []",
			},
			wantNotOutput: []string{
				"Successfully deleted",
			},
		},
		{
			name: "confirmed deletion",
			params: deleteVersionParams{
				owner:     "testowner",
				ownerType: "user",
				imageName: "testimage",
				versionID: 11111,
				tags:      []string{"test"},
				force:     false,
				dryRun:    false,
			},
			confirmResponse: true,
			wantDeleted:     true,
			wantErr:         false,
			wantOutput:      []string{"Successfully deleted version 11111"},
		},
		{
			name: "cancelled by user",
			params: deleteVersionParams{
				owner:     "testowner",
				ownerType: "user",
				imageName: "testimage",
				versionID: 22222,
				tags:      []string{},
				force:     false,
				dryRun:    false,
			},
			confirmResponse: false,
			wantDeleted:     false,
			wantErr:         false,
			wantOutput:      []string{"Deletion cancelled"},
			wantNotOutput:   []string{"Successfully deleted"},
		},
		{
			name: "confirmation error",
			params: deleteVersionParams{
				owner:     "testowner",
				ownerType: "user",
				imageName: "testimage",
				versionID: 33333,
				tags:      []string{},
				force:     false,
				dryRun:    false,
			},
			confirmErr:  fmt.Errorf("stdin closed"),
			wantDeleted: false,
			wantErr:     true,
		},
		{
			name: "delete API error",
			params: deleteVersionParams{
				owner:     "testowner",
				ownerType: "user",
				imageName: "testimage",
				versionID: 44444,
				tags:      []string{},
				force:     true,
				dryRun:    false,
			},
			deleteErr:   fmt.Errorf("permission denied"),
			wantDeleted: false,
			wantErr:     true,
		},
		{
			name: "shows graph count when present",
			params: deleteVersionParams{
				owner:      "testowner",
				ownerType:  "user",
				imageName:  "testimage",
				versionID:  55555,
				tags:       []string{},
				graphCount: 2,
				force:      true,
				dryRun:     false,
			},
			wantDeleted: true,
			wantErr:     false,
			wantOutput:  []string{"Graphs:", "2 graphs"},
		},
		{
			name: "shows singular graph when count is 1",
			params: deleteVersionParams{
				owner:      "testowner",
				ownerType:  "user",
				imageName:  "testimage",
				versionID:  66666,
				tags:       []string{},
				graphCount: 1,
				force:      true,
				dryRun:     false,
			},
			wantDeleted: true,
			wantErr:     false,
			wantOutput:  []string{"1 graph"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockPackageDeleter()
			if tt.deleteErr != nil {
				mock.deleteErrors[tt.params.versionID] = tt.deleteErr
			}

			var buf strings.Builder
			ctx := context.Background()

			confirmFn := func() (bool, error) {
				return tt.confirmResponse, tt.confirmErr
			}

			err := executeSingleDelete(ctx, mock, tt.params, &buf, confirmFn)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}

			// Check deletion
			deleted := len(mock.deletedVersions) > 0
			if deleted != tt.wantDeleted {
				t.Errorf("deleted = %v, want %v", deleted, tt.wantDeleted)
			}

			// Check output
			output := buf.String()
			for _, want := range tt.wantOutput {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\nGot:\n%s", want, output)
				}
			}
			for _, notWant := range tt.wantNotOutput {
				if strings.Contains(output, notWant) {
					t.Errorf("output should NOT contain %q\nGot:\n%s", notWant, output)
				}
			}
		})
	}
}

// =============================================================================
// Tests for executeBulkDelete
// =============================================================================

func TestExecuteBulkDelete(t *testing.T) {
	tests := []struct {
		name            string
		params          bulkDeleteParams
		confirmResponse bool
		confirmErr      error
		deleteErrors    map[int64]error
		wantDeleted     []int64
		wantErr         bool
		wantOutput      []string
		wantNotOutput   []string
	}{
		{
			name: "successful bulk deletion with force",
			params: bulkDeleteParams{
				owner:     "testowner",
				ownerType: "user",
				imageName: "testimage",
				versions: []gh.PackageVersionInfo{
					{ID: 100, Tags: []string{"v1.0"}, CreatedAt: "2025-01-01"},
					{ID: 101, Tags: []string{}, CreatedAt: "2025-01-02"},
					{ID: 102, Tags: []string{"v2.0"}, CreatedAt: "2025-01-03"},
				},
				force:  true,
				dryRun: false,
			},
			wantDeleted: []int64{100, 101, 102},
			wantErr:     false,
			wantOutput: []string{
				"Preparing to delete 3 package version(s)",
				"Image: testimage",
				"Owner: testowner (user)",
				"ID: 100, Tags: v1.0",
				"ID: 101, Tags: []",
				"ID: 102, Tags: v2.0",
				"Deleting version 1/3",
				"Deleting version 2/3",
				"Deleting version 3/3",
				"3 succeeded",
			},
		},
		{
			name: "dry run does not delete",
			params: bulkDeleteParams{
				owner:     "testowner",
				ownerType: "org",
				imageName: "testimage",
				versions: []gh.PackageVersionInfo{
					{ID: 200, Tags: []string{"test"}, CreatedAt: "2025-01-01"},
				},
				force:  false,
				dryRun: true,
			},
			wantDeleted: []int64{},
			wantErr:     false,
			wantOutput:  []string{"DRY RUN: No changes made"},
			wantNotOutput: []string{
				"Deleting version",
				"succeeded",
			},
		},
		{
			name: "cancelled by user",
			params: bulkDeleteParams{
				owner:     "testowner",
				ownerType: "user",
				imageName: "testimage",
				versions: []gh.PackageVersionInfo{
					{ID: 300, Tags: []string{}, CreatedAt: "2025-01-01"},
				},
				force:  false,
				dryRun: false,
			},
			confirmResponse: false,
			wantDeleted:     []int64{},
			wantErr:         false,
			wantOutput:      []string{"Deletion cancelled"},
		},
		{
			name: "confirmed bulk deletion",
			params: bulkDeleteParams{
				owner:     "testowner",
				ownerType: "user",
				imageName: "testimage",
				versions: []gh.PackageVersionInfo{
					{ID: 400, Tags: []string{}, CreatedAt: "2025-01-01"},
					{ID: 401, Tags: []string{}, CreatedAt: "2025-01-02"},
				},
				force:  false,
				dryRun: false,
			},
			confirmResponse: true,
			wantDeleted:     []int64{400, 401},
			wantErr:         false,
			wantOutput:      []string{"2 succeeded"},
		},
		{
			name: "partial failure continues",
			params: bulkDeleteParams{
				owner:     "testowner",
				ownerType: "user",
				imageName: "testimage",
				versions: []gh.PackageVersionInfo{
					{ID: 500, Tags: []string{}, CreatedAt: "2025-01-01"},
					{ID: 501, Tags: []string{}, CreatedAt: "2025-01-02"},
					{ID: 502, Tags: []string{}, CreatedAt: "2025-01-03"},
				},
				force:  true,
				dryRun: false,
			},
			deleteErrors: map[int64]error{501: fmt.Errorf("permission denied")},
			wantDeleted:  []int64{500, 502}, // 501 fails but others succeed
			wantErr:      true,
			wantOutput: []string{
				"2 succeeded",
				"1 failed",
				"Failed: permission denied",
			},
		},
		{
			name: "all failures",
			params: bulkDeleteParams{
				owner:     "testowner",
				ownerType: "user",
				imageName: "testimage",
				versions: []gh.PackageVersionInfo{
					{ID: 600, Tags: []string{}, CreatedAt: "2025-01-01"},
					{ID: 601, Tags: []string{}, CreatedAt: "2025-01-02"},
				},
				force:  true,
				dryRun: false,
			},
			deleteErrors: map[int64]error{
				600: fmt.Errorf("error 1"),
				601: fmt.Errorf("error 2"),
			},
			wantDeleted: []int64{},
			wantErr:     true,
			wantOutput: []string{
				"0 succeeded",
				"2 failed",
			},
		},
		{
			name: "truncates display at 10 versions",
			params: bulkDeleteParams{
				owner:     "testowner",
				ownerType: "user",
				imageName: "testimage",
				versions: []gh.PackageVersionInfo{
					{ID: 1, CreatedAt: "2025-01-01"},
					{ID: 2, CreatedAt: "2025-01-01"},
					{ID: 3, CreatedAt: "2025-01-01"},
					{ID: 4, CreatedAt: "2025-01-01"},
					{ID: 5, CreatedAt: "2025-01-01"},
					{ID: 6, CreatedAt: "2025-01-01"},
					{ID: 7, CreatedAt: "2025-01-01"},
					{ID: 8, CreatedAt: "2025-01-01"},
					{ID: 9, CreatedAt: "2025-01-01"},
					{ID: 10, CreatedAt: "2025-01-01"},
					{ID: 11, CreatedAt: "2025-01-01"},
					{ID: 12, CreatedAt: "2025-01-01"},
				},
				force:  true,
				dryRun: false,
			},
			wantDeleted: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
			wantErr:     false,
			wantOutput: []string{
				"... and 2 more",
				"12 succeeded",
			},
		},
		{
			name: "confirmation error",
			params: bulkDeleteParams{
				owner:     "testowner",
				ownerType: "user",
				imageName: "testimage",
				versions: []gh.PackageVersionInfo{
					{ID: 700, Tags: []string{}, CreatedAt: "2025-01-01"},
				},
				force:  false,
				dryRun: false,
			},
			confirmErr:  fmt.Errorf("input error"),
			wantDeleted: []int64{},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockPackageDeleter()
			if tt.deleteErrors != nil {
				mock.deleteErrors = tt.deleteErrors
			}

			var buf strings.Builder
			ctx := context.Background()

			confirmFn := func(count int) (bool, error) {
				return tt.confirmResponse, tt.confirmErr
			}

			err := executeBulkDelete(ctx, mock, tt.params, &buf, confirmFn)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}

			// Check deleted versions
			if len(mock.deletedVersions) != len(tt.wantDeleted) {
				t.Errorf("deleted %d versions, want %d: got %v", len(mock.deletedVersions), len(tt.wantDeleted), mock.deletedVersions)
			}

			// Check output
			output := buf.String()
			for _, want := range tt.wantOutput {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\nGot:\n%s", want, output)
				}
			}
			for _, notWant := range tt.wantNotOutput {
				if strings.Contains(output, notWant) {
					t.Errorf("output should NOT contain %q\nGot:\n%s", notWant, output)
				}
			}
		})
	}
}
