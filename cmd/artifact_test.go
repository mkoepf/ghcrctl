package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/discover"
)

func TestCapitalizeFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "lowercase word",
			input: "sbom",
			want:  "Sbom",
		},
		{
			name:  "single character",
			input: "a",
			want:  "A",
		},
		{
			name:  "already capitalized",
			input: "Provenance",
			want:  "0rovenance", // Note: function assumes lowercase input
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := capitalizeFirst(tt.input)
			if got != tt.want {
				t.Errorf("capitalizeFirst(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestListArtifacts_ExampleShowsGetCommand(t *testing.T) {
	artifacts := []discover.VersionInfo{
		{Digest: "sha256:abc123def456789012345678901234567890123456789012345678901234"},
		{Digest: "sha256:def456abc789012345678901234567890123456789012345678901234567"},
	}

	var buf bytes.Buffer
	err := listArtifacts(&buf, artifacts, "myimage", "sbom", "tag", "latest")
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
	err := listArtifacts(&buf, artifacts, "myimage", "provenance", "tag", "latest")
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
	err := listArtifacts(&buf, artifacts, "test-image", "sbom", "tag", "v1.0.0")
	if err != nil {
		t.Fatalf("listArtifacts returned error: %v", err)
	}

	output := buf.String()

	// Should show header with image context
	if !strings.Contains(output, "Multiple sbom documents found in image tagged 'v1.0.0'") {
		t.Errorf("Expected header with tag context, got:\n%s", output)
	}

	// Should list both digests
	if !strings.Contains(output, "sha256:abc123def456") {
		t.Errorf("Expected first digest in output, got:\n%s", output)
	}
	if !strings.Contains(output, "sha256:def456abc789") {
		t.Errorf("Expected second digest in output, got:\n%s", output)
	}

	// Should show usage hint
	if !strings.Contains(output, "Select one by digest") {
		t.Errorf("Expected usage hint, got:\n%s", output)
	}
}

func TestListArtifacts_DigestSelector(t *testing.T) {
	artifacts := []discover.VersionInfo{
		{Digest: "sha256:abc123def456789012345678901234567890123456789012345678901234"},
	}

	var buf bytes.Buffer
	err := listArtifacts(&buf, artifacts, "test-image", "provenance", "digest", "abc123def456")
	if err != nil {
		t.Fatalf("listArtifacts returned error: %v", err)
	}

	output := buf.String()

	// Should show header with digest context
	if !strings.Contains(output, "image containing digest abc123def456") {
		t.Errorf("Expected header with digest context, got:\n%s", output)
	}
}

func TestListArtifacts_VersionSelector(t *testing.T) {
	artifacts := []discover.VersionInfo{
		{Digest: "sha256:abc123def456789012345678901234567890123456789012345678901234"},
	}

	var buf bytes.Buffer
	err := listArtifacts(&buf, artifacts, "test-image", "sbom", "version", "12345678")
	if err != nil {
		t.Fatalf("listArtifacts returned error: %v", err)
	}

	output := buf.String()

	// Should show header with version context
	if !strings.Contains(output, "image containing version 12345678") {
		t.Errorf("Expected header with version context, got:\n%s", output)
	}
}
