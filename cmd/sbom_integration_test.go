//go:build !mutating

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSBOMCommandWithImage tests sbom command against real image with SBOM
func TestSBOMCommandWithImage(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "Skipping integration test - GITHUB_TOKEN not set")

	// Create fresh command instance
	// Use --all flag since the test image is multiarch and has multiple SBOMs (one per platform)
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"get", "sbom", "mkoepf/ghcrctl-test-with-sbom", "--tag", "latest", "--all"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Execute command
	err := cmd.Execute()
	require.NoError(t, err, "sbom command failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("SBOM output length: %d bytes", len(output))

	// Verify output contains SBOM data
	assert.NotEmpty(t, output, "Expected SBOM output, got empty string")

	// Should contain some indication of SBOM format (SPDX, CycloneDX, or in-toto)
	hasFormat := strings.Contains(output, "SPDX") ||
		strings.Contains(output, "CycloneDX") ||
		strings.Contains(output, "in-toto") ||
		strings.Contains(output, "predicate")

	assert.True(t, hasFormat, "Expected SBOM to contain format indicators (SPDX/CycloneDX/in-toto)")
}

// TestSBOMCommandWithoutSBOM tests sbom command against image without SBOM
func TestSBOMCommandWithoutSBOM(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "Skipping integration test - GITHUB_TOKEN not set")

	// Create fresh command instance
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"get", "sbom", "mkoepf/ghcrctl-test-no-sbom", "--tag", "latest"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Execute command - should fail
	err := cmd.Execute()
	require.Error(t, err, "Expected error when SBOM not found, got none")

	// Error should mention no SBOM found
	hasSBOMError := strings.Contains(err.Error(), "no SBOM found") || strings.Contains(err.Error(), "SBOM")
	assert.True(t, hasSBOMError, "Expected error about missing SBOM, got: %v", err)
}

// TestSBOMCommandJSONOutput tests sbom command with --json flag
func TestSBOMCommandJSONOutput(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "Skipping integration test - GITHUB_TOKEN not set")

	// Create fresh command instance
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"get", "sbom", "mkoepf/ghcrctl-test-with-sbom", "--tag", "latest", "--json"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Execute command
	err := cmd.Execute()
	require.NoError(t, err, "sbom command failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("SBOM JSON output length: %d bytes", len(output))

	// Verify output is valid JSON
	var parsed interface{}
	err = json.Unmarshal([]byte(output), &parsed)
	assert.NoError(t, err, "Output is not valid JSON: %s", output[:min(len(output), 200)])
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
