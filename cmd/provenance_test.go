package cmd

import (
	"testing"
)

func TestProvenanceCommandStructure(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	provenanceCmd, _, err := cmd.Find([]string{"provenance"})
	if err != nil {
		t.Fatalf("Failed to find provenance command: %v", err)
	}

	if provenanceCmd.Use != "provenance <image>" {
		t.Errorf("Expected Use 'provenance <image>', got '%s'", provenanceCmd.Use)
	}

	if provenanceCmd.Short == "" {
		t.Error("Expected non-empty Short description")
	}

	if provenanceCmd.Long == "" {
		t.Error("Expected non-empty Long description")
	}
}

func TestProvenanceCommandArguments(t *testing.T) {
	t.Parallel()
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
			cmd := NewRootCmd()
			provenanceCmd, _, _ := cmd.Find([]string{"provenance"})
			err := provenanceCmd.Args(provenanceCmd, tt.args)
			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestProvenanceCommandHasFlags(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	provenanceCmd, _, _ := cmd.Find([]string{"provenance"})

	flags := []string{"tag", "digest", "all", "json"}

	for _, flagName := range flags {
		flag := provenanceCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Expected flag '%s' to exist", flagName)
		}
	}

	// Check default values
	tagFlag := provenanceCmd.Flags().Lookup("tag")
	if tagFlag.DefValue != "latest" {
		t.Errorf("Expected tag default 'latest', got '%s'", tagFlag.DefValue)
	}
}

func TestShortProvenanceDigest(t *testing.T) {
	t.Parallel()
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
			result := shortProvenanceDigest(tt.digest)
			if result != tt.expected {
				t.Errorf("shortProvenanceDigest(%s) = %s, want %s", tt.digest, result, tt.expected)
			}
		})
	}
}
