package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/config"
)

// TestProvenanceCommandWithImage tests provenance command against real image with provenance
func TestProvenanceCommandWithImage(t *testing.T) {
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
	rootCmd.SetArgs([]string{"provenance", "ghcrctl-test-with-sbom", "--tag", "latest"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	// Execute command
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("provenance command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("Provenance output length: %d bytes", len(output))

	// Verify output contains provenance data
	if len(output) == 0 {
		t.Error("Expected provenance output, got empty string")
	}

	// Should contain some indication of provenance format (SLSA, in-toto, or predicate)
	hasFormat := strings.Contains(output, "SLSA") ||
		strings.Contains(output, "slsa") ||
		strings.Contains(output, "in-toto") ||
		strings.Contains(output, "predicate") ||
		strings.Contains(output, "provenance")

	if !hasFormat {
		t.Error("Expected provenance to contain format indicators (SLSA/in-toto/predicate)")
	}

	// Reset args
	rootCmd.SetArgs([]string{})
}

// TestProvenanceCommandJSONOutput tests provenance command with --json flag
func TestProvenanceCommandJSONOutput(t *testing.T) {
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
	rootCmd.SetArgs([]string{"provenance", "ghcrctl-test-with-sbom", "--tag", "latest", "--json"})

	// Reset the JSON flag explicitly
	provenanceJSON = false

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	// Execute command
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("provenance command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("Provenance JSON output length: %d bytes", len(output))

	// Verify output is valid JSON
	var parsed interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Errorf("Output is not valid JSON: %v\nOutput: %s", err, output[:min(len(output), 200)])
	}

	// Reset args and flag
	rootCmd.SetArgs([]string{})
	provenanceJSON = false
}

// TestProvenanceCommandWithBothAttestations tests that test image has both SBOM and provenance
func TestProvenanceCommandWithBothAttestations(t *testing.T) {
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

	// Test that both sbom and provenance commands work on the same image
	// This verifies that the image has both attestations

	// Test SBOM
	rootCmd.SetArgs([]string{"sbom", "ghcrctl-test-with-sbom", "--tag", "latest"})
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))

	err = rootCmd.Execute()
	if err != nil {
		t.Errorf("SBOM command failed: %v", err)
	}

	if len(stdout.String()) == 0 {
		t.Error("Expected SBOM output")
	}

	// Test Provenance
	rootCmd.SetArgs([]string{"provenance", "ghcrctl-test-with-sbom", "--tag", "latest"})
	stdout = new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))

	err = rootCmd.Execute()
	if err != nil {
		t.Errorf("Provenance command failed: %v", err)
	}

	if len(stdout.String()) == 0 {
		t.Error("Expected provenance output")
	}

	// Reset args
	rootCmd.SetArgs([]string{})
}
