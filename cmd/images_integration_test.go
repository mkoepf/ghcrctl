package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mhk/ghcrctl/internal/config"
)

// TestImagesCommandWithRepoScopedToken tests that images command fails with helpful error
// when using a repository-scoped token (like GitHub Actions GITHUB_TOKEN)
func TestImagesCommandWithRepoScopedToken(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	// Set up config
	cfg := config.New()
	err := cfg.SetOwner("mkoepf", "user")
	if err != nil {
		t.Fatalf("Failed to set owner: %v", err)
	}

	// Reset root command args
	rootCmd.SetArgs([]string{"images"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	// Execute command - should fail with repo-scoped token
	err = rootCmd.Execute()

	// We expect an error because the repo-scoped token doesn't have broad read:packages access
	if err == nil {
		t.Error("Expected error with repo-scoped token, got none")
		t.Logf("Stdout: %s", stdout.String())
		t.Logf("Stderr: %s", stderr.String())
	} else {
		t.Logf("Got expected error: %v", err)

		// Verify error message is helpful
		errMsg := err.Error()

		// Should mention one of these things to help the user understand
		hasHelpfulMessage := strings.Contains(errMsg, "fine-grained") ||
			strings.Contains(errMsg, "repository-scoped") ||
			strings.Contains(errMsg, "read:packages") ||
			strings.Contains(errMsg, "401") ||
			strings.Contains(errMsg, "403") ||
			strings.Contains(errMsg, "400") ||
			strings.Contains(strings.ToLower(errMsg), "token") ||
			strings.Contains(strings.ToLower(errMsg), "permission")

		if !hasHelpfulMessage {
			t.Errorf("Error message should be helpful about token permissions, got: %v", err)
		}
	}

	// Reset args
	rootCmd.SetArgs([]string{})
}

// TestImagesCommandErrorFormat verifies error handling without usage hint
func TestImagesCommandErrorFormat(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	// Set up config
	cfg := config.New()
	err := cfg.SetOwner("mkoepf", "user")
	if err != nil {
		t.Fatalf("Failed to set owner: %v", err)
	}

	// Reset root command args
	rootCmd.SetArgs([]string{"images"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	// Execute command
	err = rootCmd.Execute()

	// Should fail with operational error
	if err != nil {
		stderrOutput := stderr.String()

		// Should NOT show usage hint for operational errors
		// (Usage hint would contain "Usage:" or "Flags:")
		if strings.Contains(stderrOutput, "Usage:") {
			t.Error("Operational error should not show usage hint")
			t.Logf("Stderr output: %s", stderrOutput)
		}
	}

	// Reset args
	rootCmd.SetArgs([]string{})
}
