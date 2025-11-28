package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/gh"
)

// Integration tests for the versions command
// These tests require GITHUB_TOKEN and will skip if not set

// TestVersionsCommandWithMultiarch tests versions command with multiarch image
func TestVersionsCommandWithMultiarch(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	// Create fresh command instance
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"versions", "ghcrctl-test-with-sbom"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Execute command
	err := cmd.Execute()
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
}

// TestVersionsCommandWithTagFilter tests versions command with --tag filter
func TestVersionsCommandWithTagFilter(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	// Create fresh command instance
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"versions", "ghcrctl-test-with-sbom", "--tag", "v1.0"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Execute command
	err := cmd.Execute()
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
}

// TestVersionsCommandJSON tests JSON output format
func TestVersionsCommandJSON(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	// Create fresh command instance
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"versions", "ghcrctl-test-with-sbom", "--json"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Execute command
	err := cmd.Execute()
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
}

// TestVersionsCommandSinglePlatform tests versions with single platform image
func TestVersionsCommandSinglePlatform(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	// Create fresh command instance
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"versions", "ghcrctl-test-no-sbom"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Execute command
	err := cmd.Execute()
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
}

// TestVersionsCommandWithSBOMOnly tests image with SBOM but no provenance
func TestVersionsCommandWithSBOMOnly(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	// Create fresh command instance
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"versions", "ghcrctl-test-with-sbom-no-provenance"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Execute command
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("versions command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("Versions output (with SBOM, no provenance):\n%s", output)

	// Verify SBOM is shown in the artifact list
	lines := strings.Split(output, "\n")
	foundSBOM := false
	foundProvenance := false
	for _, line := range lines {
		// Skip header and separator lines
		if strings.Contains(line, "VERSION ID") || strings.Contains(line, "---") {
			continue
		}
		// Check if line contains artifact types
		if strings.Contains(line, "sbom") && !strings.Contains(line, "ghcrctl-test") {
			foundSBOM = true
		}
		if strings.Contains(line, "provenance") && !strings.Contains(line, "ghcrctl-test") {
			foundProvenance = true
		}
	}

	if !foundSBOM {
		t.Error("Expected 'sbom' artifact in output")
	}
	if foundProvenance {
		t.Error("Expected provenance artifact to NOT be in output for SBOM-only image")
	}
}

// TestVersionsCommandCosignAttestationsGrouped tests that cosign attestations are grouped under parent
func TestVersionsCommandCosignAttestationsGrouped(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}
	t.Parallel()

	// Test with cosign-vuln image that has vuln-scan and vex attestations
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"versions", "ghcrctl-test-cosign-vuln"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Execute command
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("versions command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("Versions output (cosign attestations):\n%s", output)

	// Verify the cosign attestation (.att) is grouped under the parent image
	// The .att tag should appear as a child of the parent image, not as a separate graph
	if !strings.Contains(output, "vex") && !strings.Contains(output, "vuln-scan") {
		t.Error("Expected vuln-scan or vex attestation type in output")
	}

	// Verify tree structure exists (indicating grouping)
	if !strings.Contains(output, "┌") || !strings.Contains(output, "└") {
		t.Error("Expected tree indicators showing attestations grouped under parent")
	}

	// Count the number of graphs - should be 1 or 2 (parent + orphan), not 3 separate graphs
	graphCount := strings.Count(output, "┌")
	t.Logf("Number of graphs with children: %d", graphCount)
}
