package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/config"
)

// TestImagesCommandWithRepoScopedToken tests that images command works with valid token
// Note: The tested path differs depending on the token's permissions. It is not
// possible to provide the exact same token type for local development and CI
// environments due to GitHub limitations.
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

	// Execute command
	err = rootCmd.Execute()

	// If token has sufficient permissions (access to user / org namespace and
	// read:packages), command should succeed. If it's a repo-scoped token (e.g.
	// installation token from a GitHub App) it is expected to fail due to
	// insufficient permissions.
	//
	// Both cases are handled here to allow testing in different environments.
	if err == nil {
		// Case 1:
		// In the GitHub Actions CI, a personal access token with broad
		// read:packages permissions is used, so the command should succeed
		// there.

		// Success case - token has sufficient permissions
		t.Logf("Command succeeded with current token (has read:packages access)")
		stdoutStr := stdout.String()

		// Should show at least the test images
		if !strings.Contains(stdoutStr, "ghcrctl-test") {
			t.Error("Expected to see test images in output")
			t.Logf("Stdout: %s", stdoutStr)
		}
	} else {
		// Case 2:
		// In local development, a fine-grained personal access token or a
		// GitHub App installation token with repository-scoped permissions is
		// typically used, which is expected to fail due to insufficient
		// permissions.

		// Failure case - token lacks permissions
		t.Logf("Command failed (token lacks broad read:packages): %v", err)

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
