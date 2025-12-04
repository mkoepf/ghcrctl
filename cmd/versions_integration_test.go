//go:build !mutating

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests for the versions command
// These tests require GITHUB_TOKEN and will skip if not set

// TestVersionsCommandFlat tests versions command flat table output
func TestVersionsCommandFlat(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "Skipping integration test - GITHUB_TOKEN not set")

	// Create fresh command instance - flat table output is default
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "versions", "mkoepf/ghcrctl-test-with-sbom"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Execute command
	err := cmd.Execute()
	require.NoError(t, err, "versions command failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("Versions output:\n%s", output)

	// Verify output contains expected sections
	assert.Contains(t, output, "Versions for ghcrctl-test-with-sbom")

	// Verify flat table format has headers
	assert.Contains(t, output, "VERSION ID", "Expected output to contain 'VERSION ID' header")
	assert.Contains(t, output, "DIGEST", "Expected output to contain 'DIGEST' header")
	assert.Contains(t, output, "TAGS", "Expected output to contain 'TAGS' header")
	assert.Contains(t, output, "CREATED", "Expected output to contain 'CREATED' header")

	// Verify total count
	assert.Contains(t, output, "Total:", "Expected 'Total:' in output")
}

// TestVersionsCommandWithTagFilter tests versions command with --tag filter
func TestVersionsCommandWithTagFilter(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "Skipping integration test - GITHUB_TOKEN not set")

	// Create fresh command instance
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "versions", "mkoepf/ghcrctl-test-with-sbom", "--tag", "v1.0"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Execute command
	err := cmd.Execute()
	require.NoError(t, err, "versions command failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("Versions output with --tag:\n%s", output)

	// Verify the tag "v1.0" is in the output
	assert.Contains(t, output, "v1.0", "Expected filtered tag 'v1.0' in output")
}

// TestVersionsCommandJSON tests JSON output format
func TestVersionsCommandJSON(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "Skipping integration test - GITHUB_TOKEN not set")

	// Create fresh command instance
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "versions", "mkoepf/ghcrctl-test-with-sbom", "--json"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Execute command
	err := cmd.Execute()
	require.NoError(t, err, "versions command failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("JSON output:\n%s", output)

	// Verify it's valid JSON
	var versions []gh.PackageVersionInfo
	err = json.Unmarshal([]byte(output), &versions)
	require.NoError(t, err, "Failed to parse JSON output")

	// Verify we got multiple versions
	assert.GreaterOrEqual(t, len(versions), 2, "Expected at least 2 versions")

	// Verify structure of version objects
	for _, ver := range versions {
		assert.NotZero(t, ver.ID, "Expected non-zero version ID")
		assert.NotEmpty(t, ver.Name, "Expected non-empty version name (digest)")
		assert.NotEmpty(t, ver.CreatedAt, "Expected non-empty CreatedAt timestamp")
	}
}

// TestVersionsCommandSinglePlatform tests versions with single platform image
func TestVersionsCommandSinglePlatform(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "Skipping integration test - GITHUB_TOKEN not set")

	// Create fresh command instance
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "versions", "mkoepf/ghcrctl-test-no-sbom"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Execute command
	err := cmd.Execute()
	require.NoError(t, err, "versions command failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("Versions output (single platform):\n%s", output)

	// Verify output
	assert.Contains(t, output, "Versions for ghcrctl-test-no-sbom")

	// Verify we get version output with headers
	assert.Contains(t, output, "VERSION ID", "Expected output to contain 'VERSION ID' header")
}

// Note: Hierarchical tree view tests are now in images_integration_test.go
// The versions command only shows flat table output.
// Use 'ghcrctl images' for tree view with artifact relationships.
