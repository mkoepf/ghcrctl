package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mhk/ghcrctl/internal/config"
)

// Integration tests for the graph command
// These tests require GITHUB_TOKEN and will skip if not set

// ===[ Graph Command End-to-End Tests ]===

// TestGraphCommandWithSBOM runs full graph command against image with SBOM
func TestGraphCommandWithSBOM(t *testing.T) {
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
	rootCmd.SetArgs([]string{"graph", "ghcrctl-test-with-sbom", "--tag", "latest"})

	// Capture output
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	// Execute command
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("graph command failed: %v", err)
	}

	output := buf.String()
	t.Logf("Graph output:\n%s", output)

	// Verify output contains expected sections
	if !strings.Contains(output, "OCI Artifact Graph") {
		t.Error("Expected output to contain 'OCI Artifact Graph'")
	}

	if !strings.Contains(output, "Summary:") {
		t.Error("Expected output to contain 'Summary:'")
	}

	// Verify SBOM is detected
	if !strings.Contains(output, "SBOM: true") {
		t.Error("Expected 'SBOM: true' in summary")
	}

	// Verify total versions count
	if !strings.Contains(output, "Total versions:") {
		t.Error("Expected 'Total versions:' in summary")
	}

	// Verify platform manifests are shown
	if !strings.Contains(output, "Platform Manifests") {
		t.Error("Expected 'Platform Manifests' in output")
	}

	// Reset args
	rootCmd.SetArgs([]string{})
}

// TestGraphCommandWithoutSBOM runs full graph command against plain image
func TestGraphCommandWithoutSBOM(t *testing.T) {
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
	rootCmd.SetArgs([]string{"graph", "ghcrctl-test-no-sbom", "--tag", "latest"})

	// Capture output
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	// Execute command
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("graph command failed: %v", err)
	}

	output := buf.String()
	t.Logf("Graph output:\n%s", output)

	// Verify SBOM is not detected
	if !strings.Contains(output, "SBOM: false") {
		t.Error("Expected 'SBOM: false' in summary")
	}

	// Note: The "no-sbom" image may still have provenance, so we just verify SBOM is false
	// Total versions will be >= 1
	if !strings.Contains(output, "Total versions:") {
		t.Error("Expected 'Total versions:' in summary")
	}

	// Reset args
	rootCmd.SetArgs([]string{})
}

// TestGraphCommandJSONOutput tests graph command with --json flag
func TestGraphCommandJSONOutput(t *testing.T) {
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
	rootCmd.SetArgs([]string{"graph", "ghcrctl-test-with-sbom", "--tag", "latest", "--json"})

	// Capture output - separate stdout and stderr
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	// Execute command
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("graph command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	if stderr.Len() > 0 {
		t.Logf("Warnings/errors:\n%s", stderr.String())
	}
	t.Logf("JSON output:\n%s", output)

	// Parse JSON - use only stdout
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	if err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Verify JSON structure
	if _, ok := result["root"]; !ok {
		t.Error("Expected JSON to have 'root' field")
	}

	if _, ok := result["referrers"]; !ok {
		t.Error("Expected JSON to have 'referrers' field")
	}

	// Verify root structure
	root, ok := result["root"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'root' to be an object")
	}

	if _, ok := root["digest"]; !ok {
		t.Error("Expected root to have 'digest' field")
	}

	if _, ok := root["tags"]; !ok {
		t.Error("Expected root to have 'tags' field")
	}

	// Verify referrers is an array
	referrers, ok := result["referrers"].([]interface{})
	if !ok {
		t.Fatal("Expected 'referrers' to be an array")
	}

	if len(referrers) == 0 {
		t.Error("Expected at least one referrer for image with SBOM")
	}

	t.Logf("✓ JSON structure validated, found %d referrers", len(referrers))

	// Reset args
	rootCmd.SetArgs([]string{})
}

// ===[ Output Format Tests ]===

// TestGraphTableOutputFormat verifies human-readable table output
func TestGraphTableOutputFormat(t *testing.T) {
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

	// Reset root command args (no --json flag)
	rootCmd.SetArgs([]string{"graph", "ghcrctl-test-with-sbom", "--tag", "latest"})

	// Reset the JSON flag explicitly (it may persist from previous tests)
	graphJSONOutput = false

	// Capture output - separate stdout and stderr
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	// Execute command
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("graph command failed: %v", err)
	}

	output := stdout.String()
	t.Logf("Table output:\n%s", output)

	// Verify expected sections are present
	expectedSections := []string{
		"OCI Artifact Graph for ghcrctl-test-with-sbom",
		"Image Index:",
		"Digest:",
		"Platform Manifests",
		"Attestations",
		"Summary:",
		"SBOM:",
		"Provenance:",
		"Total versions:",
	}

	for _, section := range expectedSections {
		if !strings.Contains(output, section) {
			t.Errorf("Expected table output to contain '%s'", section)
		}
	}

	t.Logf("✓ Table output format verified")

	// Reset args
	rootCmd.SetArgs([]string{})
}

// TestGraphJSONOutputStructure validates JSON schema compliance
func TestGraphJSONOutputStructure(t *testing.T) {
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
	rootCmd.SetArgs([]string{"graph", "ghcrctl-test-with-sbom", "--tag", "latest", "--json"})

	// Capture output - separate stdout and stderr
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	// Execute command
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("graph command failed: %v", err)
	}

	output := stdout.String()

	// Parse and validate JSON structure
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Validate root object structure
	root, ok := result["root"].(map[string]interface{})
	if !ok {
		t.Fatal("root must be an object")
	}

	requiredRootFields := []string{"digest", "type", "tags", "version_id"}
	for _, field := range requiredRootFields {
		if _, ok := root[field]; !ok {
			t.Errorf("root missing required field: %s", field)
		}
	}

	// Validate referrers array
	referrers, ok := result["referrers"].([]interface{})
	if !ok {
		t.Fatal("referrers must be an array")
	}

	// Each referrer should have required fields
	for i, ref := range referrers {
		refObj, ok := ref.(map[string]interface{})
		if !ok {
			t.Errorf("referrer[%d] must be an object", i)
			continue
		}

		requiredRefFields := []string{"digest", "type", "tags", "version_id"}
		for _, field := range requiredRefFields {
			if _, ok := refObj[field]; !ok {
				t.Errorf("referrer[%d] missing required field: %s", i, field)
			}
		}
	}

	t.Logf("✓ JSON schema validated successfully")

	// Reset args
	rootCmd.SetArgs([]string{})
}

// TestGraphShowsAllTagsForDigest verifies that graph displays all tags pointing to a digest
func TestGraphShowsAllTagsForDigest(t *testing.T) {
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

	// Note: This test assumes ghcrctl-test-with-sbom has been tagged with both "latest" and "newest"
	// pointing to the same digest (via: ghcrctl tag ghcrctl-test-with-sbom latest newest)

	// Test querying with "latest" tag - should show ALL tags for that digest
	rootCmd.SetArgs([]string{"graph", "ghcrctl-test-with-sbom", "--tag", "latest"})
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("graph command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	t.Logf("Graph output for --tag latest:\n%s", output)

	// Verify both tags are shown
	if !strings.Contains(output, "latest") {
		t.Error("Expected output to contain 'latest' tag")
	}
	if !strings.Contains(output, "newest") {
		t.Error("Expected output to contain 'newest' tag (all tags for digest should be shown)")
	}

	// Test with JSON output as well
	rootCmd.SetArgs([]string{"graph", "ghcrctl-test-with-sbom", "--tag", "latest", "--json"})
	stdout.Reset()
	stderr.Reset()
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("graph command failed: %v", err)
	}

	jsonOutput := stdout.String()
	var result map[string]interface{}
	err = json.Unmarshal([]byte(jsonOutput), &result)
	if err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	root, ok := result["root"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'root' to be an object")
	}

	tags, ok := root["tags"].([]interface{})
	if !ok {
		t.Fatal("Expected 'tags' to be an array")
	}

	// Verify tags array contains both "latest" and "newest"
	tagStrings := make([]string, len(tags))
	for i, tag := range tags {
		tagStrings[i] = tag.(string)
	}

	hasLatest := false
	hasNewest := false
	for _, tag := range tagStrings {
		if tag == "latest" {
			hasLatest = true
		}
		if tag == "newest" {
			hasNewest = true
		}
	}

	if !hasLatest {
		t.Errorf("Expected tags to contain 'latest', got: %v", tagStrings)
	}
	if !hasNewest {
		t.Errorf("Expected tags to contain 'newest', got: %v", tagStrings)
	}

	t.Logf("✓ Graph correctly shows all tags for digest: %v", tagStrings)

	// Reset args
	rootCmd.SetArgs([]string{})
}
