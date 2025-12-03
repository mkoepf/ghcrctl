package cmd

import (
	"bytes"
	"testing"
)

// TestLabelsCommandStructure verifies the labels command is properly set up
func TestLabelsCommandStructure(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	labelsCmd, _, err := cmd.Find([]string{"get", "labels"})
	if err != nil {
		t.Fatalf("Failed to find get labels command: %v", err)
	}

	if labelsCmd.Use != "labels <owner/package>" {
		t.Errorf("Expected Use 'labels <owner/package>', got '%s'", labelsCmd.Use)
	}

	if labelsCmd.Short == "" {
		t.Error("Short description should not be empty")
	}
}

// TestLabelsCommandArguments verifies argument validation
func TestLabelsCommandArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		args        []string
		expectUsage bool
	}{
		{
			name:        "missing image argument",
			args:        []string{"get", "labels"},
			expectUsage: true,
		},
		{
			name:        "too many arguments",
			args:        []string{"get", "labels", "myimage", "extra"},
			expectUsage: true,
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewRootCmd()
			cmd.SetArgs(tt.args)

			// Capture output
			stdout := new(bytes.Buffer)
			stderr := new(bytes.Buffer)
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)

			err := cmd.Execute()

			// Should fail with usage error
			if err == nil {
				t.Error("Expected error but got none")
			}
		})
	}
}

// TestLabelsCommandHasFlags verifies required flags exist
func TestLabelsCommandHasFlags(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	labelsCmd, _, err := cmd.Find([]string{"get", "labels"})
	if err != nil {
		t.Fatalf("Failed to find get labels command: %v", err)
	}

	// Check for selector flags
	flags := []string{"tag", "digest", "version", "key", "json"}
	for _, flagName := range flags {
		flag := labelsCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Expected --%s flag to exist", flagName)
		}
	}
}
