//go:build !mutating

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests for the stats command
// These tests require GITHUB_TOKEN and will skip if not set

func TestStatsCommand_TableOutput(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"stats", "mkoepf/ghcrctl-test-with-sbom"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.NoError(t, err, "stats command failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("Stats output:\n%s", output)

	// Verify header
	assert.Contains(t, output, "Statistics for ghcrctl-test-with-sbom")

	// Verify key statistics are present
	expectedFields := []string{
		"Total versions:",
		"Tagged versions:",
		"Untagged versions:",
		"Total tags:",
		"Oldest version:",
		"Newest version:",
	}

	for _, field := range expectedFields {
		assert.Contains(t, output, field, "Expected output to contain '%s'", field)
	}
}

func TestStatsCommand_JSONOutput(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"stats", "mkoepf/ghcrctl-test-with-sbom", "--json"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.NoError(t, err, "stats command failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("JSON output:\n%s", output)

	// Verify it's valid JSON
	var stats PackageStats
	err = json.Unmarshal([]byte(output), &stats)
	require.NoError(t, err, "Failed to parse JSON output")

	// Verify structure
	assert.Equal(t, "ghcrctl-test-with-sbom", stats.PackageName)
	assert.GreaterOrEqual(t, stats.TotalVersions, 1, "Expected at least 1 total version")
	assert.GreaterOrEqual(t, stats.TaggedVersions, 1, "Expected at least 1 tagged version")
}

func TestStatsCommand_SinglePlatformImage(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"stats", "mkoepf/ghcrctl-test-no-sbom"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.NoError(t, err, "stats command failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("Stats output (single platform):\n%s", output)

	// Verify statistics are present
	assert.Contains(t, output, "Total versions:", "Expected output to contain 'Total versions:'")
}

func TestStatsCommand_InvalidPackage(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"stats", "mkoepf/nonexistent-package-12345"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.Error(t, err, "Expected error for nonexistent package, got none")

	// Should be an operational error, not show usage
	assert.NotContains(t, stderr.String(), "Usage:", "Operational error should not show usage hint")
}
