//go:build !mutating

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

// TestVersionsCommandFlat tests versions command flat table output
func TestVersionsCommandFlat(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

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
	if err != nil {
		t.Fatalf("versions command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("Versions output:\n%s", output)

	// Verify output contains expected sections
	if !strings.Contains(output, "Versions for ghcrctl-test-with-sbom") {
		t.Error("Expected output to contain 'Versions for ghcrctl-test-with-sbom'")
	}

	// Verify flat table format has headers
	if !strings.Contains(output, "VERSION ID") {
		t.Error("Expected output to contain 'VERSION ID' header")
	}
	if !strings.Contains(output, "DIGEST") {
		t.Error("Expected output to contain 'DIGEST' header")
	}
	if !strings.Contains(output, "TAGS") {
		t.Error("Expected output to contain 'TAGS' header")
	}
	if !strings.Contains(output, "CREATED") {
		t.Error("Expected output to contain 'CREATED' header")
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
	cmd.SetArgs([]string{"list", "versions", "mkoepf/ghcrctl-test-with-sbom", "--tag", "v1.0"})

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
	cmd.SetArgs([]string{"list", "versions", "mkoepf/ghcrctl-test-with-sbom", "--json"})

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

	// Verify we got multiple versions
	if len(versions) < 2 {
		t.Errorf("Expected at least 2 versions, got %d", len(versions))
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
	cmd.SetArgs([]string{"list", "versions", "mkoepf/ghcrctl-test-no-sbom"})

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

	// Verify we get version output with headers
	if !strings.Contains(output, "VERSION ID") {
		t.Error("Expected output to contain 'VERSION ID' header")
	}
}

// Note: Hierarchical tree view tests are now in images_integration_test.go
// The versions command only shows flat table output.
// Use 'ghcrctl images' for tree view with artifact relationships.
