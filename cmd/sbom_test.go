package cmd

import (
	"bytes"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/display"
)

func TestGetSBOMCommandStructure(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	sbomCmd, _, err := cmd.Find([]string{"get", "sbom"})
	if err != nil {
		t.Fatalf("Failed to find get sbom command: %v", err)
	}

	if sbomCmd.Use != "sbom <owner/package>" {
		t.Errorf("Expected Use 'sbom <owner/package>', got '%s'", sbomCmd.Use)
	}

	if sbomCmd.Short == "" {
		t.Error("Expected non-empty Short description")
	}

	if sbomCmd.Long == "" {
		t.Error("Expected non-empty Long description")
	}
}

func TestGetSBOMCommandArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		args      []string
		wantError bool
	}{
		{
			name:      "missing image argument",
			args:      []string{"get", "sbom"},
			wantError: true,
		},
		{
			name:      "valid single argument with owner/package",
			args:      []string{"get", "sbom", "mkoepf/test-image"},
			wantError: false, // Will fail for other reasons (no selector flag), but not arg count
		},
		{
			name:      "too many arguments",
			args:      []string{"get", "sbom", "image1", "image2"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCmd()
			var out bytes.Buffer
			var errOut bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&errOut)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			// For valid args count, it may still fail but not due to arg count
			if !tt.wantError && err != nil {
				// Check if it's an args error
				errStr := err.Error()
				if errStr == "accepts 1 arg(s), received 0" || errStr == "accepts 1 arg(s), received 2" {
					t.Errorf("Unexpected arg count error: %v", err)
				}
			}
		})
	}
}

func TestGetSBOMCommandHasFlags(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	sbomCmd, _, _ := cmd.Find([]string{"get", "sbom"})

	// Check for required flags
	flags := []string{"tag", "digest", "sbom-digest", "all", "json"}

	for _, flagName := range flags {
		flag := sbomCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Expected flag '%s' to exist", flagName)
		}
	}
}

func TestShortDigest(t *testing.T) {
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
			result := display.ShortDigest(tt.digest)
			if result != tt.expected {
				t.Errorf("display.ShortDigest(%s) = %s, want %s", tt.digest, result, tt.expected)
			}
		})
	}
}
