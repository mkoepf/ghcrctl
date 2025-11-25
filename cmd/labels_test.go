package cmd

import (
	"testing"
)

// TestLabelsCommandStructure verifies the labels command is properly set up
func TestLabelsCommandStructure(t *testing.T) {
	cmd := rootCmd
	labelsCmd, _, err := cmd.Find([]string{"labels"})
	if err != nil {
		t.Fatalf("Failed to find labels command: %v", err)
	}

	if labelsCmd.Use != "labels <image>" {
		t.Errorf("Expected Use 'labels <image>', got '%s'", labelsCmd.Use)
	}

	if labelsCmd.Short == "" {
		t.Error("Short description should not be empty")
	}
}

// TestLabelsCommandArguments verifies argument validation
func TestLabelsCommandArguments(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectUsage bool
	}{
		{
			name:        "missing image argument",
			args:        []string{"labels"},
			expectUsage: true,
		},
		{
			name:        "too many arguments",
			args:        []string{"labels", "myimage", "extra"},
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

// TestLabelsCommandHasFlags verifies required flags exist
func TestLabelsCommandHasFlags(t *testing.T) {
	cmd := rootCmd
	labelsCmd, _, err := cmd.Find([]string{"labels"})
	if err != nil {
		t.Fatalf("Failed to find labels command: %v", err)
	}

	// Check for --tag flag
	tagFlag := labelsCmd.Flags().Lookup("tag")
	if tagFlag == nil {
		t.Error("Expected --tag flag to exist")
	}

	// Check for --key flag
	keyFlag := labelsCmd.Flags().Lookup("key")
	if keyFlag == nil {
		t.Error("Expected --key flag to exist")
	}

	// Check for --json flag
	jsonFlag := labelsCmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Error("Expected --json flag to exist")
	}
}
