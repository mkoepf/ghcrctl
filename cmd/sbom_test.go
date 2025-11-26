package cmd

import (
	"testing"

	"github.com/mhk/ghcrctl/internal/display"
)

func TestSBOMCommandStructure(t *testing.T) {
	if sbomCmd.Use != "sbom <image>" {
		t.Errorf("Expected Use 'sbom <image>', got '%s'", sbomCmd.Use)
	}

	if sbomCmd.Short == "" {
		t.Error("Expected non-empty Short description")
	}

	if sbomCmd.Long == "" {
		t.Error("Expected non-empty Long description")
	}
}

func TestSBOMCommandArguments(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantError bool
	}{
		{
			name:      "missing image argument",
			args:      []string{},
			wantError: true,
		},
		{
			name:      "valid single argument",
			args:      []string{"test-image"},
			wantError: false,
		},
		{
			name:      "too many arguments",
			args:      []string{"image1", "image2"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sbomCmd.Args(sbomCmd, tt.args)
			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestSBOMCommandHasFlags(t *testing.T) {
	flags := []string{"tag", "digest", "all", "json"}

	for _, flagName := range flags {
		flag := sbomCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Expected flag '%s' to exist", flagName)
		}
	}

	// Check default values
	tagFlag := sbomCmd.Flags().Lookup("tag")
	if tagFlag.DefValue != "latest" {
		t.Errorf("Expected tag default 'latest', got '%s'", tagFlag.DefValue)
	}
}

func TestShortDigest(t *testing.T) {
	tests := []struct {
		name     string
		digest   string
		expected string
	}{
		{
			name:     "full sha256 digest",
			digest:   "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expected: "1234567890ab",
		},
		{
			name:     "short digest",
			digest:   "sha256:abc",
			expected: "abc",
		},
		{
			name:     "no prefix",
			digest:   "1234567890abcdef",
			expected: "1234567890ab",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := display.ShortDigest(tt.digest)
			if result != tt.expected {
				t.Errorf("display.ShortDigest(%s) = %s, want %s", tt.digest, result, tt.expected)
			}
		})
	}
}
