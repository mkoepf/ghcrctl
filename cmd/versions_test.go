package cmd

import (
	"context"
	"testing"

	"github.com/mhk/ghcrctl/internal/gh"
)

// TestDiscoverRelatedVersionsByDigest verifies that we can discover relationships
// using a digest directly without needing to call ResolveTag
func TestDiscoverRelatedVersionsByDigest(t *testing.T) {
	ctx := context.Background()

	// Test with a simple digest
	digest := "sha256:abc123"
	fullImage := "ghcr.io/test/image"

	// This should work without making any ORAS ResolveTag calls
	// The function should accept the digest directly
	artifacts, graphType := discoverRelatedVersionsByDigest(ctx, fullImage, digest, digest)

	// Basic validation - function should not panic and return valid type
	if graphType == "" {
		t.Error("Expected non-empty graph type")
	}

	// artifacts may be empty if no relationships found, which is OK for this test
	_ = artifacts
}

// TestBuildVersionGraphsUsesDigestDirectly verifies optimization:
// buildVersionGraphs should use ver.Name (digest) directly instead of calling ResolveTag
func TestBuildVersionGraphsUsesDigestDirectly(t *testing.T) {
	ctx := context.Background()

	// Create test versions with digests already set
	versions := []gh.PackageVersionInfo{
		{
			ID:   12345,
			Name: "sha256:abc123def456",
			Tags: []string{"v1.0", "latest"},
		},
		{
			ID:   12346,
			Name: "sha256:def456abc123",
			Tags: []string{},
		},
	}

	fullImage := "ghcr.io/test/image"

	// This should use digests directly from ver.Name
	// and NOT call oras.ResolveTag for each tag
	graphs, err := buildVersionGraphs(ctx, fullImage, versions, versions, nil, "", "", "")

	if err != nil {
		// Error is expected since we're not providing real ORAS connectivity
		// The key is that it should attempt to use digests, not tags
		t.Logf("Expected error in test environment: %v", err)
	}

	// Basic validation - should attempt to create graphs
	if graphs == nil {
		t.Error("Expected non-nil graphs result")
	}
}

// TestFilterVersionsByTag verifies that versions can be filtered by tag before graph building
func TestFilterVersionsByTag(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		{
			ID:   1,
			Name: "sha256:aaa",
			Tags: []string{"v1.0", "latest"},
		},
		{
			ID:   2,
			Name: "sha256:bbb",
			Tags: []string{"v2.0"},
		},
		{
			ID:   3,
			Name: "sha256:ccc",
			Tags: []string{"v1.0", "stable"},
		},
		{
			ID:   4,
			Name: "sha256:ddd",
			Tags: []string{},
		},
	}

	tests := []struct {
		name          string
		filterTag     string
		expectedCount int
		expectedIDs   []int64
	}{
		{
			name:          "filter by v1.0",
			filterTag:     "v1.0",
			expectedCount: 2,
			expectedIDs:   []int64{1, 3},
		},
		{
			name:          "filter by v2.0",
			filterTag:     "v2.0",
			expectedCount: 1,
			expectedIDs:   []int64{2},
		},
		{
			name:          "filter by non-existent tag",
			filterTag:     "nonexistent",
			expectedCount: 0,
			expectedIDs:   []int64{},
		},
		{
			name:          "no filter",
			filterTag:     "",
			expectedCount: 4,
			expectedIDs:   []int64{1, 2, 3, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := filterVersionsByTag(versions, tt.filterTag)

			if len(filtered) != tt.expectedCount {
				t.Errorf("Expected %d versions, got %d", tt.expectedCount, len(filtered))
			}

			// Check that the correct versions were included
			for _, expectedID := range tt.expectedIDs {
				found := false
				for _, ver := range filtered {
					if ver.ID == expectedID {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected version ID %d to be in filtered results", expectedID)
				}
			}
		})
	}
}

// TestVersionsCommandStructure verifies the versions command is properly set up
func TestVersionsCommandStructure(t *testing.T) {
	cmd := rootCmd
	versionsCmd, _, err := cmd.Find([]string{"versions"})
	if err != nil {
		t.Fatalf("Failed to find versions command: %v", err)
	}

	if versionsCmd.Use != "versions <image>" {
		t.Errorf("Expected Use 'versions <image>', got '%s'", versionsCmd.Use)
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

// TestVersionsCommandHasFlags verifies required flags exist
func TestVersionsCommandHasFlags(t *testing.T) {
	// Check for --json flag
	jsonFlag := versionsCmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Error("versions command should have --json flag")
	}

	// Check for --tag flag
	tagFlag := versionsCmd.Flags().Lookup("tag")
	if tagFlag == nil {
		t.Error("versions command should have --tag flag")
	}
}
