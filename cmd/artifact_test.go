package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/discover"
)

func TestListArtifacts_ExampleShowsGetCommand(t *testing.T) {
	artifacts := []discover.VersionInfo{
		{Digest: "sha256:abc123def456789012345678901234567890123456789012345678901234"},
		{Digest: "sha256:def456abc789012345678901234567890123456789012345678901234567"},
	}

	var buf bytes.Buffer
	err := listArtifacts(&buf, artifacts, "myimage", "sbom")
	if err != nil {
		t.Fatalf("listArtifacts returned error: %v", err)
	}

	output := buf.String()

	// The example should show "ghcrctl get sbom" not "ghcrctl sbom"
	if !strings.Contains(output, "ghcrctl get sbom") {
		t.Errorf("Expected example to show 'ghcrctl get sbom', got:\n%s", output)
	}

	// Should not show the incorrect command without "get"
	if strings.Contains(output, "Example: ghcrctl sbom") {
		t.Errorf("Example should not show 'ghcrctl sbom' (missing 'get'), got:\n%s", output)
	}
}

func TestListArtifacts_ExampleShowsGetProvenance(t *testing.T) {
	artifacts := []discover.VersionInfo{
		{Digest: "sha256:abc123def456789012345678901234567890123456789012345678901234"},
	}

	var buf bytes.Buffer
	err := listArtifacts(&buf, artifacts, "myimage", "provenance")
	if err != nil {
		t.Fatalf("listArtifacts returned error: %v", err)
	}

	output := buf.String()

	// The example should show "ghcrctl get provenance" not "ghcrctl provenance"
	if !strings.Contains(output, "ghcrctl get provenance") {
		t.Errorf("Expected example to show 'ghcrctl get provenance', got:\n%s", output)
	}
}

func TestListArtifacts_OutputFormat(t *testing.T) {
	artifacts := []discover.VersionInfo{
		{Digest: "sha256:abc123def456789012345678901234567890123456789012345678901234"},
		{Digest: "sha256:def456abc789012345678901234567890123456789012345678901234567"},
	}

	var buf bytes.Buffer
	err := listArtifacts(&buf, artifacts, "test-image", "sbom")
	if err != nil {
		t.Fatalf("listArtifacts returned error: %v", err)
	}

	output := buf.String()

	// Should show header
	if !strings.Contains(output, "Multiple sbom documents found for test-image") {
		t.Errorf("Expected header with image name, got:\n%s", output)
	}

	// Should list both digests
	if !strings.Contains(output, "sha256:abc123def456") {
		t.Errorf("Expected first digest in output, got:\n%s", output)
	}
	if !strings.Contains(output, "sha256:def456abc789") {
		t.Errorf("Expected second digest in output, got:\n%s", output)
	}

	// Should show usage hint
	if !strings.Contains(output, "Use --digest <digest> to select one") {
		t.Errorf("Expected usage hint, got:\n%s", output)
	}
}
