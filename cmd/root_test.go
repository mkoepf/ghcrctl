package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommand(t *testing.T) {
	t.Parallel()
	// Test root command structure
	cmd := NewRootCmd()

	if cmd.Use != "ghcrctl" {
		t.Errorf("Expected Use to be 'ghcrctl', got '%s'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be non-empty")
	}

	if cmd.Long == "" {
		t.Error("Expected Long description to be non-empty")
	}

	// Verify Long description contains expected functionality
	expectedKeywords := []string{
		"images",
		"OCI",
		"metadata",
		"deletion",
	}

	for _, keyword := range expectedKeywords {
		if !strings.Contains(cmd.Long, keyword) {
			t.Errorf("Expected Long description to contain '%s'", keyword)
		}
	}
}

func TestRootCommandHasImagesSubcommand(t *testing.T) {
	t.Parallel()
	// Verify images subcommand is registered
	cmd := NewRootCmd()
	found := false
	for _, c := range cmd.Commands() {
		if strings.HasPrefix(c.Use, "images") {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected images subcommand to be registered with root command")
	}
}

func TestRootCommandHelp(t *testing.T) {
	t.Parallel()
	// Test that help command works
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"--help"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()

	// Help returns nil error but sets flag
	if err != nil {
		t.Errorf("Expected --help to succeed, got error: %v", err)
	}
}

func TestRootCommandVersion(t *testing.T) {
	t.Parallel()
	// Test that --version flag is available and returns version info
	cmd := NewRootCmd()

	if cmd.Version == "" {
		t.Error("cmd.Version should not be empty")
	}

	// Version should be set (either "dev" or injected at build time)
	if cmd.Version != "dev" && !strings.HasPrefix(cmd.Version, "v") {
		t.Errorf("Expected version to be 'dev' or start with 'v', got %q", cmd.Version)
	}
}
