package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestLabelsCommandWithRealImage tests the labels command with a real image
func TestLabelsCommandWithRealImage(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	// Test with the ghcrctl-test-with-sbom image which should have labels
	image := "mkoepf/ghcrctl-test-with-sbom"

	// Create fresh command instance
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"get", "labels", image, "--tag", "latest"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Execute command
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("Labels output:\n%s", output)

	// Verify output contains expected elements
	if !strings.Contains(output, "Labels for") {
		t.Error("Expected output to contain 'Labels for'")
	}

	if !strings.Contains(output, "Total:") {
		t.Error("Expected output to contain 'Total:'")
	}
}

// TestLabelsCommandJSON tests JSON output format
func TestLabelsCommandJSON(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	image := "mkoepf/ghcrctl-test-with-sbom"

	// Create fresh command instance
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"get", "labels", image, "--tag", "latest", "--json"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Execute command
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("JSON output:\n%s", output)

	// Verify it looks like JSON
	if !strings.HasPrefix(strings.TrimSpace(output), "{") {
		t.Error("Expected JSON output to start with '{'")
	}

	if !strings.HasSuffix(strings.TrimSpace(output), "}") {
		t.Error("Expected JSON output to end with '}'")
	}
}

// TestLabelsCommandWithKey tests filtering by specific key
func TestLabelsCommandWithKey(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	image := "mkoepf/ghcrctl-test-with-sbom"

	// Create fresh command instance
	// Test with a common OCI label key
	// Note: We don't know which labels exist, so we'll just verify the command works
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"get", "labels", image, "--tag", "latest", "--key", "org.opencontainers.image.source"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Execute command - may fail if label doesn't exist
	err := cmd.Execute()

	output := stdout.String()
	stderrStr := stderr.String()

	// Either succeeds with the label or fails with "not found" error
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Unexpected error: %v\nStderr: %s", err, stderrStr)
		}
		t.Logf("Label key not found (expected): %v", err)
	} else {
		t.Logf("Label output:\n%s", output)
		// If successful, should show only one label
		if !strings.Contains(output, "org.opencontainers.image.source") {
			t.Error("Expected output to contain the requested label key")
		}
	}
}
