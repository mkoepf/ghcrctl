//go:build !mutating

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Integration tests for the stats command
// These tests require GITHUB_TOKEN and will skip if not set

func TestStatsCommand_TableOutput(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Fatal("GITHUB_TOKEN not set")
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"stats", "mkoepf/ghcrctl-test-with-sbom"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("stats command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("Stats output:\n%s", output)

	// Verify header
	if !strings.Contains(output, "Statistics for ghcrctl-test-with-sbom") {
		t.Error("Expected output to contain 'Statistics for ghcrctl-test-with-sbom'")
	}

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
		if !strings.Contains(output, field) {
			t.Errorf("Expected output to contain '%s'", field)
		}
	}
}

func TestStatsCommand_JSONOutput(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Fatal("GITHUB_TOKEN not set")
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"stats", "mkoepf/ghcrctl-test-with-sbom", "--json"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("stats command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("JSON output:\n%s", output)

	// Verify it's valid JSON
	var stats PackageStats
	err = json.Unmarshal([]byte(output), &stats)
	if err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Verify structure
	if stats.PackageName != "ghcrctl-test-with-sbom" {
		t.Errorf("Expected package_name='ghcrctl-test-with-sbom', got %q", stats.PackageName)
	}

	if stats.TotalVersions < 1 {
		t.Errorf("Expected at least 1 total version, got %d", stats.TotalVersions)
	}

	if stats.TaggedVersions < 1 {
		t.Errorf("Expected at least 1 tagged version, got %d", stats.TaggedVersions)
	}
}

func TestStatsCommand_SinglePlatformImage(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Fatal("GITHUB_TOKEN not set")
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"stats", "mkoepf/ghcrctl-test-no-sbom"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("stats command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("Stats output (single platform):\n%s", output)

	// Verify statistics are present
	if !strings.Contains(output, "Total versions:") {
		t.Error("Expected output to contain 'Total versions:'")
	}
}

func TestStatsCommand_InvalidPackage(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Fatal("GITHUB_TOKEN not set")
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"stats", "mkoepf/nonexistent-package-12345"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for nonexistent package, got none")
	}

	// Should be an operational error, not show usage
	if strings.Contains(stderr.String(), "Usage:") {
		t.Error("Operational error should not show usage hint")
	}
}
