package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mhk/ghcrctl/internal/config"
	"github.com/mhk/ghcrctl/internal/gh"
)

// Integration tests for the versions command
// These tests require GITHUB_TOKEN and will skip if not set

// TestVersionsCommandWithMultiarch tests versions command with multiarch image
func TestVersionsCommandWithMultiarch(t *testing.T) {
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

	// Test with the multiarch image with SBOM and provenance
	rootCmd.SetArgs([]string{"versions", "ghcrctl-test-with-sbom"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	// Execute command
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("versions command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("Versions output:\n%s", output)

	// Verify output contains expected sections
	if !strings.Contains(output, "Versions for ghcrctl-test-with-sbom") {
		t.Error("Expected output to contain 'Versions for ghcrctl-test-with-sbom'")
	}

	// Verify graph structure with tree indicators
	if !strings.Contains(output, "┌") || !strings.Contains(output, "└") {
		t.Error("Expected output to contain tree indicators (┌ └)")
	}

	// Verify type column shows "index" for multiarch root
	if !strings.Contains(output, "index") {
		t.Error("Expected 'index' type for multiarch image")
	}

	// Verify platform manifests are shown (expecting linux/amd64 and linux/arm64)
	if !strings.Contains(output, "linux/amd64") && !strings.Contains(output, "linux/arm64") {
		t.Error("Expected platform manifests (linux/amd64 or linux/arm64) in output")
	}

	// Verify attestations (sbom, provenance) are shown
	if !strings.Contains(output, "sbom") && !strings.Contains(output, "provenance") {
		t.Error("Expected sbom or provenance in output")
	}

	// Verify total count
	if !strings.Contains(output, "Total:") {
		t.Error("Expected 'Total:' in output")
	}

	// Reset args
	rootCmd.SetArgs([]string{})
}

// TestVersionsCommandWithTagFilter tests versions command with --tag filter
func TestVersionsCommandWithTagFilter(t *testing.T) {
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

	// Test with --tag filter
	rootCmd.SetArgs([]string{"versions", "ghcrctl-test-with-sbom", "--tag", "v1.0"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	// Execute command
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("versions command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("Versions output with --tag:\n%s", output)

	// Verify the output still shows the full graph (root + children)
	if !strings.Contains(output, "┌") {
		t.Error("Expected output to contain tree structure even with --tag filter")
	}

	// Verify the tag "v1.0" is in the output
	if !strings.Contains(output, "v1.0") {
		t.Error("Expected filtered tag 'v1.0' in output")
	}

	// Reset args
	rootCmd.SetArgs([]string{})
}

// TestVersionsCommandJSON tests JSON output format
func TestVersionsCommandJSON(t *testing.T) {
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

	// Test JSON output
	rootCmd.SetArgs([]string{"versions", "ghcrctl-test-with-sbom", "--json"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	// Execute command
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("versions command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("JSON output:\n%s", output)

	// Verify it's valid JSON
	var versions []gh.PackageVersionInfo
	err = json.Unmarshal([]byte(output), &versions)
	if err != nil {
		t.Errorf("Failed to parse JSON output: %v", err)
	}

	// Verify we got multiple versions (should include root + children)
	if len(versions) < 2 {
		t.Errorf("Expected at least 2 versions (root + children), got %d", len(versions))
	}

	// Verify structure of version objects
	for _, ver := range versions {
		if ver.ID == 0 {
			t.Error("Expected non-zero version ID")
		}
		if ver.Name == "" {
			t.Error("Expected non-empty version name (digest)")
		}
		if ver.CreatedAt == "" {
			t.Error("Expected non-empty CreatedAt timestamp")
		}
	}

	// Reset args
	rootCmd.SetArgs([]string{})
}

// TestVersionsCommandSinglePlatform tests versions with single platform image
func TestVersionsCommandSinglePlatform(t *testing.T) {
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

	// Test with single platform image (no SBOM, no provenance)
	rootCmd.SetArgs([]string{"versions", "ghcrctl-test-no-sbom"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	// Execute command
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("versions command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("Versions output (single platform):\n%s", output)

	// Verify output
	if !strings.Contains(output, "Versions for ghcrctl-test-no-sbom") {
		t.Error("Expected output to contain 'Versions for ghcrctl-test-no-sbom'")
	}

	// Single platform images may not have tree indicators if no children
	// Just verify we get some version output
	if !strings.Contains(output, "VERSION ID") {
		t.Error("Expected output to contain 'VERSION ID' header")
	}

	// Reset args
	rootCmd.SetArgs([]string{})
}

// TestVersionsCommandWithSBOMOnly tests image with SBOM but no provenance
func TestVersionsCommandWithSBOMOnly(t *testing.T) {
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

	// Test with image that has SBOM but no provenance
	rootCmd.SetArgs([]string{"versions", "ghcrctl-test-with-sbom-no-provenance"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	// Execute command
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("versions command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("Versions output (SBOM only):\n%s", output)

	// Verify SBOM is shown
	if !strings.Contains(output, "sbom") {
		t.Error("Expected 'sbom' in output")
	}

	// Verify provenance is NOT shown (since image doesn't have it)
	if strings.Contains(output, "provenance") {
		t.Error("Did not expect 'provenance' in output for image without provenance")
	}

	// Reset args
	rootCmd.SetArgs([]string{})
}
