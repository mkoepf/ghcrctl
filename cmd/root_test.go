package cmd

import (
	"strings"
	"testing"
)

func TestRootCommand(t *testing.T) {
	// Test root command structure
	if rootCmd.Use != "ghcrctl" {
		t.Errorf("Expected Use to be 'ghcrctl', got '%s'", rootCmd.Use)
	}

	if rootCmd.Short == "" {
		t.Error("Expected Short description to be non-empty")
	}

	if rootCmd.Long == "" {
		t.Error("Expected Long description to be non-empty")
	}

	// Verify Long description contains expected functionality
	expectedKeywords := []string{
		"images",
		"OCI",
		"metadata",
		"deletion",
		"Configuration",
	}

	for _, keyword := range expectedKeywords {
		if !strings.Contains(rootCmd.Long, keyword) {
			t.Errorf("Expected Long description to contain '%s'", keyword)
		}
	}
}

func TestRootCommandHasConfigSubcommand(t *testing.T) {
	// Verify config subcommand is registered
	found := false
	for _, cmd := range rootCmd.Commands() {
		if strings.HasPrefix(cmd.Use, "config") {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected config subcommand to be registered with root command")
	}
}

func TestRootCommandHelp(t *testing.T) {
	// Test that help command works
	rootCmd.SetArgs([]string{"--help"})
	err := rootCmd.Execute()

	// Help returns nil error but sets flag
	if err != nil {
		t.Errorf("Expected --help to succeed, got error: %v", err)
	}

	// Reset
	rootCmd.SetArgs([]string{})
}

func TestRootCommandVersion(t *testing.T) {
	// Test that --version flag is available and returns version info
	if rootCmd.Version == "" {
		t.Error("rootCmd.Version should not be empty")
	}

	// Version should be set (either "dev" or injected at build time)
	if rootCmd.Version != "dev" && !strings.HasPrefix(rootCmd.Version, "v") {
		t.Errorf("Expected version to be 'dev' or start with 'v', got %q", rootCmd.Version)
	}
}
