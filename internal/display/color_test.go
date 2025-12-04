package display

import (
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
)

func init() {
	// Disable color for predictable test output
	color.NoColor = true
}

func TestColorVersionType(t *testing.T) {
	tests := []struct {
		name        string
		versionType string
		expected    string
	}{
		{
			name:        "index type",
			versionType: "index",
			expected:    "index",
		},
		{
			name:        "manifest type",
			versionType: "manifest",
			expected:    "manifest",
		},
		{
			name:        "linux/amd64 platform",
			versionType: "linux/amd64",
			expected:    "linux/amd64",
		},
		{
			name:        "linux/arm64 platform",
			versionType: "linux/arm64",
			expected:    "linux/arm64",
		},
		{
			name:        "sbom attestation",
			versionType: "sbom",
			expected:    "sbom",
		},
		{
			name:        "provenance attestation",
			versionType: "provenance",
			expected:    "provenance",
		},
		{
			name:        "attestation type",
			versionType: "attestation",
			expected:    "attestation",
		},
		{
			name:        "platform prefix",
			versionType: "platform: linux/amd64",
			expected:    "platform: linux/amd64",
		},
		{
			name:        "attestation prefix",
			versionType: "attestation: sbom",
			expected:    "attestation: sbom",
		},
		{
			name:        "signature type",
			versionType: "signature",
			expected:    "signature",
		},
		{
			name:        "vuln-scan type",
			versionType: "vuln-scan",
			expected:    "vuln-scan",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ColorVersionType(tt.versionType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestColorTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		expected string
	}{
		{
			name:     "empty tags",
			tags:     []string{},
			expected: "[]",
		},
		{
			name:     "nil tags",
			tags:     nil,
			expected: "[]",
		},
		{
			name:     "single tag",
			tags:     []string{"v1.0.0"},
			expected: "[v1.0.0]",
		},
		{
			name:     "multiple tags",
			tags:     []string{"v1.0.0", "latest"},
			expected: "[v1.0.0, latest]",
		},
		{
			name:     "latest tag",
			tags:     []string{"latest"},
			expected: "[latest]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ColorTags(tt.tags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestColorDigest(t *testing.T) {
	tests := []struct {
		name     string
		digest   string
		expected string
	}{
		{
			name:     "short digest",
			digest:   "abcdef123456",
			expected: "abcdef123456",
		},
		{
			name:     "full digest",
			digest:   "sha256:abcdef1234567890",
			expected: "sha256:abcdef1234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ColorDigest(tt.digest)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestColorHeader(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "version id header",
			header:   "VERSION ID",
			expected: "VERSION ID",
		},
		{
			name:     "type header",
			header:   "TYPE",
			expected: "TYPE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ColorHeader(tt.header)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestColorSeparator(t *testing.T) {
	tests := []struct {
		name      string
		separator string
		expected  string
	}{
		{
			name:      "dashes",
			separator: "----------",
			expected:  "----------",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ColorSeparator(tt.separator)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestColorSuccess(t *testing.T) {
	result := ColorSuccess("Successfully deleted")
	assert.Equal(t, "Successfully deleted", result)
}

func TestColorWarning(t *testing.T) {
	result := ColorWarning("Are you sure?")
	assert.Equal(t, "Are you sure?", result)
}

func TestColorError(t *testing.T) {
	result := ColorError("Failed to delete")
	assert.Equal(t, "Failed to delete", result)
}

func TestColorDryRun(t *testing.T) {
	result := ColorDryRun("DRY RUN: No changes made")
	assert.Equal(t, "DRY RUN: No changes made", result)
}

func TestColorCount(t *testing.T) {
	result := ColorCount(42)
	assert.Equal(t, "42", result)
}
