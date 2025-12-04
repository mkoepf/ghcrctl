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

// TestProvenanceCommandWithImage tests provenance command against real image with provenance
func TestProvenanceCommandWithImage(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "Skipping integration test - GITHUB_TOKEN not set")

	// Create fresh command instance
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"get", "provenance", "mkoepf/ghcrctl-test-with-sbom", "--tag", "latest"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Execute command
	err := cmd.Execute()
	require.NoError(t, err, "provenance command failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("Provenance output length: %d bytes", len(output))

	// Verify output contains provenance data
	assert.NotEmpty(t, output, "Expected provenance output, got empty string")

	// Should contain some indication of provenance format (SLSA, in-toto, or predicate)
	hasFormat := strings.Contains(output, "SLSA") ||
		strings.Contains(output, "slsa") ||
		strings.Contains(output, "in-toto") ||
		strings.Contains(output, "predicate") ||
		strings.Contains(output, "provenance")

	assert.True(t, hasFormat, "Expected provenance to contain format indicators (SLSA/in-toto/predicate)")
}

// TestProvenanceCommandJSONOutput tests provenance command with --json flag
func TestProvenanceCommandJSONOutput(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "Skipping integration test - GITHUB_TOKEN not set")

	// Create fresh command instance
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"get", "provenance", "mkoepf/ghcrctl-test-with-sbom", "--tag", "latest", "--json"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Execute command
	err := cmd.Execute()
	require.NoError(t, err, "provenance command failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("Provenance JSON output length: %d bytes", len(output))

	// Verify output is valid JSON
	var parsed interface{}
	err = json.Unmarshal([]byte(output), &parsed)
	assert.NoError(t, err, "Output is not valid JSON: %s", output[:min(len(output), 200)])
}

// TestProvenanceCommandWithBothAttestations tests that test image has both SBOM and provenance
func TestProvenanceCommandWithBothAttestations(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "Skipping integration test - GITHUB_TOKEN not set")

	// Test that both sbom and provenance commands work on the same image
	// This verifies that the image has both attestations

	// Test SBOM
	sbomCmd := NewRootCmd()
	sbomCmd.SetArgs([]string{"get", "sbom", "mkoepf/ghcrctl-test-with-sbom", "--tag", "latest"})
	stdout := new(bytes.Buffer)
	sbomCmd.SetOut(stdout)
	sbomCmd.SetErr(new(bytes.Buffer))

	err := sbomCmd.Execute()
	assert.NoError(t, err, "SBOM command failed")
	assert.NotEmpty(t, stdout.String(), "Expected SBOM output")

	// Test Provenance
	provCmd := NewRootCmd()
	provCmd.SetArgs([]string{"get", "provenance", "mkoepf/ghcrctl-test-with-sbom", "--tag", "latest"})
	stdout = new(bytes.Buffer)
	provCmd.SetOut(stdout)
	provCmd.SetErr(new(bytes.Buffer))

	err = provCmd.Execute()
	assert.NoError(t, err, "Provenance command failed")
	assert.NotEmpty(t, stdout.String(), "Expected provenance output")
}
