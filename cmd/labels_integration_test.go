//go:build !mutating

package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLabelsCommandWithRealImage tests the labels command with a real image
func TestLabelsCommandWithRealImage(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "Skipping integration test - GITHUB_TOKEN not set")

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
	require.NoError(t, err, "Command failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("Labels output:\n%s", output)

	// Verify output contains expected elements
	assert.Contains(t, output, "Labels for", "Expected output to contain 'Labels for'")
	assert.Contains(t, output, "Total:", "Expected output to contain 'Total:'")
}

// TestLabelsCommandJSON tests JSON output format
func TestLabelsCommandJSON(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "Skipping integration test - GITHUB_TOKEN not set")

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
	require.NoError(t, err, "Command failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("JSON output:\n%s", output)

	// Verify it looks like JSON
	assert.True(t, strings.HasPrefix(strings.TrimSpace(output), "{"), "Expected JSON output to start with '{'")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(output), "}"), "Expected JSON output to end with '}'")
}

// TestLabelsCommandWithKey tests filtering by specific key
func TestLabelsCommandWithKey(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "Skipping integration test - GITHUB_TOKEN not set")

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
		assert.ErrorContains(t, err, "not found", "Unexpected error: %v\nStderr: %s", err, stderrStr)
		t.Logf("Label key not found (expected): %v", err)
	} else {
		t.Logf("Label output:\n%s", output)
		// If successful, should show only one label
		assert.Contains(t, output, "org.opencontainers.image.source", "Expected output to contain the requested label key")
	}
}
