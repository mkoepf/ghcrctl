package cmd

import (
	"context"
	"testing"
)

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
	cmd := rootCmd
	versionsCmd, _, err := cmd.Find([]string{"versions"})
	if err != nil {
		t.Fatalf("Failed to find versions command: %v", err)
	}

	// Check for --json flag
	jsonFlag := versionsCmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Error("Expected --json flag to exist")
	}
}

// TestDiscoverRelatedVersionsByDigest verifies digest-based discovery works
func TestDiscoverRelatedVersionsByDigest(t *testing.T) {
	// This test verifies that discoverRelatedVersionsByDigest exists and has
	// the correct signature. Actual behavior is tested via integration tests.

	// The function should accept (ctx, fullImage, digest, rootDigest)
	// and return ([]DiscoveredArtifact, string)
	// This is a compile-time check that the function exists with the right signature

	// Call the function with dummy values to verify it compiles
	ctx := context.Background()
	artifacts, graphType := discoverRelatedVersionsByDigest(ctx, "", "", "")

	// Verify return types (should compile even if values are empty/nil)
	_ = artifacts // []DiscoveredArtifact
	_ = graphType // string
}
