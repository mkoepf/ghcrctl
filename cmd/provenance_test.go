package cmd

import (
	"bytes"
	"testing"
)

func TestGetProvenanceCommandStructure(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	provenanceCmd, _, err := cmd.Find([]string{"get", "provenance"})
	if err != nil {
		t.Fatalf("Failed to find get provenance command: %v", err)
	}

	if provenanceCmd.Use != "provenance <owner/package>" {
		t.Errorf("Expected Use 'provenance <owner/package>', got '%s'", provenanceCmd.Use)
	}

	if provenanceCmd.Short == "" {
		t.Error("Expected non-empty Short description")
	}

	if provenanceCmd.Long == "" {
		t.Error("Expected non-empty Long description")
	}
}

func TestGetProvenanceCommandArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		args      []string
		wantError bool
	}{
		{
			name:      "missing image argument",
			args:      []string{"get", "provenance"},
			wantError: true,
		},
		{
			name:      "valid single argument with owner/package",
			args:      []string{"get", "provenance", "mkoepf/test-image"},
			wantError: false, // Will fail for other reasons (no selector flag), but not arg count
		},
		{
			name:      "too many arguments",
			args:      []string{"get", "provenance", "image1", "image2"},
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

func TestGetProvenanceCommandHasFlags(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	provenanceCmd, _, _ := cmd.Find([]string{"get", "provenance"})

	// Check for required flags
	flags := []string{"tag", "digest", "provenance-digest", "all", "json"}

	for _, flagName := range flags {
		flag := provenanceCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Expected flag '%s' to exist", flagName)
		}
	}
}

// Note: shortProvenanceDigest was removed - now using display.ShortDigest
// The functionality is tested in internal/display/formatter_test.go
