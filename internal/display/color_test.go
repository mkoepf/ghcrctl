package display

import (
	"testing"

	"github.com/fatih/color"
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ColorVersionType(tt.versionType)
			if result != tt.expected {
				t.Errorf("ColorVersionType(%q) = %q, expected %q", tt.versionType, result, tt.expected)
			}
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
			if result != tt.expected {
				t.Errorf("ColorTags(%v) = %q, expected %q", tt.tags, result, tt.expected)
			}
		})
	}
}

func TestColorTreeIndicator(t *testing.T) {
	tests := []struct {
		name      string
		indicator string
		expected  string
	}{
		{
			name:      "root indicator",
			indicator: "┌",
			expected:  "┌",
		},
		{
			name:      "mid indicator",
			indicator: "├",
			expected:  "├",
		},
		{
			name:      "last indicator",
			indicator: "└",
			expected:  "└",
		},
		{
			name:      "vertical line",
			indicator: "│",
			expected:  "│",
		},
		{
			name:      "space",
			indicator: " ",
			expected:  " ",
		},
		{
			name:      "horizontal line",
			indicator: "─",
			expected:  "─",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ColorTreeIndicator(tt.indicator)
			if result != tt.expected {
				t.Errorf("ColorTreeIndicator(%q) = %q, expected %q", tt.indicator, result, tt.expected)
			}
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
			if result != tt.expected {
				t.Errorf("ColorDigest(%q) = %q, expected %q", tt.digest, result, tt.expected)
			}
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
			if result != tt.expected {
				t.Errorf("ColorHeader(%q) = %q, expected %q", tt.header, result, tt.expected)
			}
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
			if result != tt.expected {
				t.Errorf("ColorSeparator(%q) = %q, expected %q", tt.separator, result, tt.expected)
			}
		})
	}
}

func TestColorSuccess(t *testing.T) {
	result := ColorSuccess("Successfully deleted")
	expected := "Successfully deleted"
	if result != expected {
		t.Errorf("ColorSuccess() = %q, expected %q", result, expected)
	}
}

func TestColorWarning(t *testing.T) {
	result := ColorWarning("Are you sure?")
	expected := "Are you sure?"
	if result != expected {
		t.Errorf("ColorWarning() = %q, expected %q", result, expected)
	}
}

func TestColorError(t *testing.T) {
	result := ColorError("Failed to delete")
	expected := "Failed to delete"
	if result != expected {
		t.Errorf("ColorError() = %q, expected %q", result, expected)
	}
}

func TestColorDryRun(t *testing.T) {
	result := ColorDryRun("DRY RUN: No changes made")
	expected := "DRY RUN: No changes made"
	if result != expected {
		t.Errorf("ColorDryRun() = %q, expected %q", result, expected)
	}
}

func TestColorCount(t *testing.T) {
	result := ColorCount(42)
	expected := "42"
	if result != expected {
		t.Errorf("ColorCount(42) = %q, expected %q", result, expected)
	}
}
