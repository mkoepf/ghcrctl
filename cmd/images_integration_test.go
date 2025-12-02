//go:build !mutating

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/discover"
)

// Integration tests for the list images command
// These tests require GITHUB_TOKEN and will skip if not set

func TestListImagesCommand_TreeOutput(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Fatal("GITHUB_TOKEN not set")
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "images", "mkoepf/ghcrctl-test-with-sbom"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list images command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("Tree output:\n%s", output)

	// Verify table headers
	if !strings.Contains(output, "VERSION ID") {
		t.Error("Expected output to contain 'VERSION ID' header")
	}
	if !strings.Contains(output, "DIGEST") {
		t.Error("Expected output to contain 'DIGEST' header")
	}

	// Tree output should have tree indicators
	if !strings.Contains(output, "├") && !strings.Contains(output, "└") {
		t.Error("Expected tree output to contain tree indicators (├ or └)")
	}

	// Should contain tags
	if !strings.Contains(output, "v1.0") {
		t.Error("Expected output to contain tag 'v1.0'")
	}
}

func TestListImagesCommand_FlatOutput(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Fatal("GITHUB_TOKEN not set")
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "images", "mkoepf/ghcrctl-test-with-sbom", "--flat"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list images command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("Flat output:\n%s", output)

	// Flat output should have table headers
	if !strings.Contains(output, "DIGEST") {
		t.Error("Expected flat output to contain 'DIGEST' header")
	}
	if !strings.Contains(output, "TYPE") {
		t.Error("Expected flat output to contain 'TYPE' header")
	}
	if !strings.Contains(output, "TAGS") {
		t.Error("Expected flat output to contain 'TAGS' header")
	}
}

func TestListImagesCommand_JSONOutput(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Fatal("GITHUB_TOKEN not set")
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "images", "mkoepf/ghcrctl-test-with-sbom", "--json"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list images command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("JSON output:\n%s", output[:min(500, len(output))])

	// Verify it's valid JSON array
	var images []discover.VersionInfo
	err = json.Unmarshal([]byte(output), &images)
	if err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Verify we got images
	if len(images) < 1 {
		t.Errorf("Expected at least 1 image, got %d", len(images))
	}

	// Verify structure of image objects
	for _, img := range images {
		if img.Digest == "" {
			t.Error("Expected non-empty digest")
		}
		if len(img.Types) == 0 {
			t.Error("Expected non-empty types")
		}
	}
}

func TestListImagesCommand_FilterByTag(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Fatal("GITHUB_TOKEN not set")
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "images", "mkoepf/ghcrctl-test-with-sbom", "--tag", "v1.0"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list images command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("Filtered output:\n%s", output)

	// Should contain the tag we filtered for
	if !strings.Contains(output, "v1.0") {
		t.Error("Expected output to contain filtered tag 'v1.0'")
	}
}

func TestListImagesCommand_OutputFormat(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Fatal("GITHUB_TOKEN not set")
	}

	// Test -o json (alternative to --json)
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "images", "mkoepf/ghcrctl-test-no-sbom", "-o", "json"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list images command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()

	// Should be valid JSON
	var images []discover.VersionInfo
	err = json.Unmarshal([]byte(output), &images)
	if err != nil {
		t.Fatalf("Failed to parse JSON output with -o flag: %v", err)
	}
}

func TestListImagesCommand_SinglePlatformImage(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Fatal("GITHUB_TOKEN not set")
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "images", "mkoepf/ghcrctl-test-no-sbom"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list images command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("Single platform output:\n%s", output)

	// Should still produce output with headers
	if !strings.Contains(output, "VERSION ID") {
		t.Error("Expected output to contain 'VERSION ID' header")
	}
	if !strings.Contains(output, "Total:") {
		t.Error("Expected output to contain 'Total:' summary")
	}
}

func TestListImagesCommand_InvalidOutputFormat(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Fatal("GITHUB_TOKEN not set")
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "images", "mkoepf/ghcrctl-test-no-sbom", "-o", "invalid"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for invalid output format, got none")
	}

	if !strings.Contains(err.Error(), "invalid output format") {
		t.Errorf("Expected error about invalid output format, got: %v", err)
	}
}

func TestListImagesCommand_InvalidPackage(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Fatal("GITHUB_TOKEN not set")
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "images", "mkoepf/nonexistent-package-12345"})

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
