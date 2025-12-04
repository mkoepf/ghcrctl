//go:build !mutating

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/discover"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests for the list graphs command
// These tests require GITHUB_TOKEN and will skip if not set

func TestListGraphsCommand_TreeOutput(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "graphs", "mkoepf/ghcrctl-test-with-sbom"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.NoError(t, err, "list graphs command failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("Tree output:\n%s", output)

	// Verify table headers
	assert.Contains(t, output, "VERSION ID", "Expected output to contain 'VERSION ID' header")
	assert.Contains(t, output, "DIGEST", "Expected output to contain 'DIGEST' header")

	// Tree output should have tree indicators
	hasTreeIndicators := strings.Contains(output, "├") || strings.Contains(output, "└")
	assert.True(t, hasTreeIndicators, "Expected tree output to contain tree indicators (├ or └)")

	// Should contain tags
	assert.Contains(t, output, "v1.0", "Expected output to contain tag 'v1.0'")
}

func TestListGraphsCommand_FlatOutput(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "graphs", "mkoepf/ghcrctl-test-with-sbom", "--flat"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.NoError(t, err, "list graphs command failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("Flat output:\n%s", output)

	// Flat output should have table headers
	assert.Contains(t, output, "DIGEST", "Expected flat output to contain 'DIGEST' header")
	assert.Contains(t, output, "TYPE", "Expected flat output to contain 'TYPE' header")
	assert.Contains(t, output, "TAGS", "Expected flat output to contain 'TAGS' header")
}

func TestListGraphsCommand_JSONOutput(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "graphs", "mkoepf/ghcrctl-test-with-sbom", "--json"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.NoError(t, err, "list graphs command failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("JSON output:\n%s", output[:min(500, len(output))])

	// Verify it's valid JSON array
	var images []discover.VersionInfo
	err = json.Unmarshal([]byte(output), &images)
	require.NoError(t, err, "Failed to parse JSON output")

	// Verify we got images
	assert.GreaterOrEqual(t, len(images), 1, "Expected at least 1 image")

	// Verify structure of image objects
	for _, img := range images {
		assert.NotEmpty(t, img.Digest, "Expected non-empty digest")
		assert.NotEmpty(t, img.Types, "Expected non-empty types")
	}
}

func TestListGraphsCommand_FilterByTag(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "graphs", "mkoepf/ghcrctl-test-with-sbom", "--tag", "v1.0"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.NoError(t, err, "list graphs command failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("Filtered output:\n%s", output)

	// Should contain the tag we filtered for
	assert.Contains(t, output, "v1.0", "Expected output to contain filtered tag 'v1.0'")
}

func TestListGraphsCommand_OutputFormat(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	// Test -o json (alternative to --json)
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "graphs", "mkoepf/ghcrctl-test-no-sbom", "-o", "json"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.NoError(t, err, "list graphs command failed: %s", stderr.String())

	output := stdout.String()

	// Should be valid JSON
	var images []discover.VersionInfo
	err = json.Unmarshal([]byte(output), &images)
	require.NoError(t, err, "Failed to parse JSON output with -o flag")
}

func TestListGraphsCommand_SinglePlatformImage(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "graphs", "mkoepf/ghcrctl-test-no-sbom"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.NoError(t, err, "list graphs command failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("Single platform output:\n%s", output)

	// Should still produce output with headers
	assert.Contains(t, output, "VERSION ID", "Expected output to contain 'VERSION ID' header")
	assert.Contains(t, output, "Total:", "Expected output to contain 'Total:' summary")
}

func TestListGraphsCommand_InvalidOutputFormat(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "graphs", "mkoepf/ghcrctl-test-no-sbom", "-o", "invalid"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.Error(t, err, "Expected error for invalid output format, got none")
	assert.ErrorContains(t, err, "invalid output format")
}

func TestListGraphsCommand_InvalidPackage(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "graphs", "mkoepf/nonexistent-package-12345"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.Error(t, err, "Expected error for nonexistent package, got none")

	// Should be an operational error, not show usage
	assert.NotContains(t, stderr.String(), "Usage:", "Operational error should not show usage hint")
}
