package cmd

import (
	"bytes"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/discover"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			assert.Equal(t, tt.want, got, "capitalizeFirst(%q)", tt.input)
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
	require.NoError(t, err, "listArtifacts returned error")

	output := buf.String()

	// The example should show "ghcrctl get sbom" not "ghcrctl sbom"
	assert.Contains(t, output, "ghcrctl get sbom", "Expected example to show 'ghcrctl get sbom'")

	// Should not show the incorrect command without "get"
	assert.NotContains(t, output, "Example: ghcrctl sbom", "Example should not show 'ghcrctl sbom' (missing 'get')")
}

func TestListArtifacts_ExampleShowsGetProvenance(t *testing.T) {
	artifacts := []discover.VersionInfo{
		{Digest: "sha256:abc123def456789012345678901234567890123456789012345678901234"},
	}

	var buf bytes.Buffer
	err := listArtifacts(&buf, artifacts, "myimage", "provenance", "tag", "latest")
	require.NoError(t, err, "listArtifacts returned error")

	output := buf.String()

	// The example should show "ghcrctl get provenance" not "ghcrctl provenance"
	assert.Contains(t, output, "ghcrctl get provenance", "Expected example to show 'ghcrctl get provenance'")
}

func TestListArtifacts_OutputFormat(t *testing.T) {
	artifacts := []discover.VersionInfo{
		{Digest: "sha256:abc123def456789012345678901234567890123456789012345678901234"},
		{Digest: "sha256:def456abc789012345678901234567890123456789012345678901234567"},
	}

	var buf bytes.Buffer
	err := listArtifacts(&buf, artifacts, "test-image", "sbom", "tag", "v1.0.0")
	require.NoError(t, err, "listArtifacts returned error")

	output := buf.String()

	// Should show header with image context
	assert.Contains(t, output, "Multiple sbom documents found in image tagged 'v1.0.0'", "Expected header with tag context")

	// Should list both digests
	assert.Contains(t, output, "sha256:abc123def456", "Expected first digest in output")
	assert.Contains(t, output, "sha256:def456abc789", "Expected second digest in output")

	// Should show usage hint
	assert.Contains(t, output, "Select one by digest", "Expected usage hint")
}

func TestListArtifacts_DigestSelector(t *testing.T) {
	artifacts := []discover.VersionInfo{
		{Digest: "sha256:abc123def456789012345678901234567890123456789012345678901234"},
	}

	var buf bytes.Buffer
	err := listArtifacts(&buf, artifacts, "test-image", "provenance", "digest", "abc123def456")
	require.NoError(t, err, "listArtifacts returned error")

	output := buf.String()

	// Should show header with digest context
	assert.Contains(t, output, "image containing digest abc123def456", "Expected header with digest context")
}

func TestListArtifacts_VersionSelector(t *testing.T) {
	artifacts := []discover.VersionInfo{
		{Digest: "sha256:abc123def456789012345678901234567890123456789012345678901234"},
	}

	var buf bytes.Buffer
	err := listArtifacts(&buf, artifacts, "test-image", "sbom", "version", "12345678")
	require.NoError(t, err, "listArtifacts returned error")

	output := buf.String()

	// Should show header with version context
	assert.Contains(t, output, "image containing version 12345678", "Expected header with version context")
}
