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

	if deleteCmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	// Check for subcommands
	subcommands := deleteCmd.Commands()
	if len(subcommands) < 2 {
		t.Errorf("Expected at least 2 subcommands (version, graph), got %d", len(subcommands))
	}
}

// TestDeleteVersionCommandStructure verifies the delete version subcommand
func TestDeleteVersionCommandStructure(t *testing.T) {
	cmd := rootCmd
	deleteVersionCmd, _, err := cmd.Find([]string{"delete", "version"})
	if err != nil {
		t.Fatalf("Failed to find delete version command: %v", err)
	}

	if deleteVersionCmd.Use != "version <image> <version-id>" {
		t.Errorf("Expected Use 'version <image> <version-id>', got '%s'", deleteVersionCmd.Use)
	}

	if deleteVersionCmd.Short == "" {
		t.Error("Short description should not be empty")
	}
}

// TestDeleteVersionCommandArguments verifies argument validation
func TestDeleteVersionCommandArguments(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectUsage bool
	}{
		{
			name:        "missing all arguments",
			args:        []string{"delete", "version"},
			expectUsage: true,
		},
		{
			name:        "missing version-id argument",
			args:        []string{"delete", "version", "myimage"},
			expectUsage: true,
		},
		{
			name:        "too many arguments",
			args:        []string{"delete", "version", "myimage", "12345", "extra"},
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

// TestDeleteVersionCommandHasFlags verifies required flags exist
func TestDeleteVersionCommandHasFlags(t *testing.T) {
	cmd := rootCmd
	deleteVersionCmd, _, err := cmd.Find([]string{"delete", "version"})
	if err != nil {
		t.Fatalf("Failed to find delete version command: %v", err)
	}

	// Check for --force flag
	forceFlag := deleteVersionCmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Error("Expected --force flag to exist")
	}

	// Check for --dry-run flag
	dryRunFlag := deleteVersionCmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Error("Expected --dry-run flag to exist")
	}

	// Check for --digest flag
	digestFlag := deleteVersionCmd.Flags().Lookup("digest")
	if digestFlag == nil {
		t.Error("Expected --digest flag to exist")
	}
}

// TestDeleteGraphCommandStructure verifies the delete graph subcommand
func TestDeleteGraphCommandStructure(t *testing.T) {
	cmd := rootCmd
	deleteGraphCmd, _, err := cmd.Find([]string{"delete", "graph"})
	if err != nil {
		t.Fatalf("Failed to find delete graph command: %v", err)
	}

	if deleteGraphCmd.Use != "graph <image> <tag>" {
		t.Errorf("Expected Use 'graph <image> <tag>', got '%s'", deleteGraphCmd.Use)
	}

	if deleteGraphCmd.Short == "" {
		t.Error("Short description should not be empty")
	}
}

// TestDeleteGraphCommandArguments verifies argument validation
func TestDeleteGraphCommandArguments(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectUsage bool
	}{
		{
			name:        "missing all arguments",
			args:        []string{"delete", "graph"},
			expectUsage: true,
		},
		{
			name:        "missing tag argument",
			args:        []string{"delete", "graph", "myimage"},
			expectUsage: true,
		},
		{
			name:        "too many arguments",
			args:        []string{"delete", "graph", "myimage", "v1.0.0", "extra"},
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

// TestDeleteGraphCommandHasFlags verifies required flags exist
func TestDeleteGraphCommandHasFlags(t *testing.T) {
	cmd := rootCmd
	deleteGraphCmd, _, err := cmd.Find([]string{"delete", "graph"})
	if err != nil {
		t.Fatalf("Failed to find delete graph command: %v", err)
	}

	// Check for --force flag
	forceFlag := deleteGraphCmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Error("Expected --force flag to exist")
	}

	// Check for --dry-run flag
	dryRunFlag := deleteGraphCmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Error("Expected --dry-run flag to exist")
	}

	// Check for --digest flag
	digestFlag := deleteGraphCmd.Flags().Lookup("digest")
	if digestFlag == nil {
		t.Error("Expected --digest flag to exist")
	}

	// Check for --version flag
	versionFlag := deleteGraphCmd.Flags().Lookup("version")
	if versionFlag == nil {
		t.Error("Expected --version flag to exist")
	}
}

// TestDeleteGraphCommandFlagExclusivity verifies mutually exclusive flags
func TestDeleteGraphCommandFlagExclusivity(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		expectErr bool
	}{
		{
			name:      "tag and digest flags both set",
			args:      []string{"delete", "graph", "myimage", "v1.0.0", "--digest", "sha256:abc"},
			expectErr: true,
		},
		{
			name:      "tag and version flags both set",
			args:      []string{"delete", "graph", "myimage", "v1.0.0", "--version", "12345"},
			expectErr: true,
		},
		{
			name:      "digest and version flags both set",
			args:      []string{"delete", "graph", "myimage", "--digest", "sha256:abc", "--version", "12345"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			if tt.expectErr && err == nil {
				t.Error("Expected error for mutually exclusive flags, got none")
			}

			// Reset args
			rootCmd.SetArgs([]string{})
		})
	}
}
