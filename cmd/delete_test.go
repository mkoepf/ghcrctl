package cmd

import (
	"testing"
)

// TestDeleteCommandStructure verifies the delete command is properly set up
func TestDeleteCommandStructure(t *testing.T) {
	cmd := rootCmd
	deleteCmd, _, err := cmd.Find([]string{"delete"})
	if err != nil {
		t.Fatalf("Failed to find delete command: %v", err)
	}

	if deleteCmd.Use != "delete <image> <version-id>" {
		t.Errorf("Expected Use 'delete <image> <version-id>', got '%s'", deleteCmd.Use)
	}

	if deleteCmd.Short == "" {
		t.Error("Short description should not be empty")
	}
}

// TestDeleteCommandArguments verifies argument validation
func TestDeleteCommandArguments(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectUsage bool
	}{
		{
			name:        "missing all arguments",
			args:        []string{"delete"},
			expectUsage: true,
		},
		{
			name:        "missing version-id argument",
			args:        []string{"delete", "myimage"},
			expectUsage: true,
		},
		{
			name:        "too many arguments",
			args:        []string{"delete", "myimage", "12345", "extra"},
			expectUsage: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			// Should fail with usage error
			if err == nil {
				t.Error("Expected error but got none")
			}

			// Reset args
			rootCmd.SetArgs([]string{})
		})
	}
}

// TestDeleteCommandHasFlags verifies required flags exist
func TestDeleteCommandHasFlags(t *testing.T) {
	cmd := rootCmd
	deleteCmd, _, err := cmd.Find([]string{"delete"})
	if err != nil {
		t.Fatalf("Failed to find delete command: %v", err)
	}

	// Check for --force flag
	forceFlag := deleteCmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Error("Expected --force flag to exist")
	}

	// Check for --dry-run flag
	dryRunFlag := deleteCmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Error("Expected --dry-run flag to exist")
	}
}
