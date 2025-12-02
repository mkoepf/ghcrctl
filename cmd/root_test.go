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
		"packages",
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

func TestRootCommandHasListSubcommand(t *testing.T) {
	t.Parallel()
	// Verify list subcommand is registered (contains packages, versions, images)
	cmd := NewRootCmd()
	listCmd, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatalf("Failed to find list subcommand: %v", err)
	}

	// Verify list has packages subcommand
	packagesFound := false
	for _, c := range listCmd.Commands() {
		if strings.HasPrefix(c.Use, "packages") {
			packagesFound = true
			break
		}
	}

	if !packagesFound {
		t.Error("Expected packages subcommand under list command")
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

func TestRootCommandOutputIncludesVersion(t *testing.T) {
	t.Parallel()
	// Test that running ghcrctl without args shows version in output
	cmd := NewRootCmd()
	cmd.SetArgs([]string{})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	_ = cmd.Execute()

	output := stdout.String()
	if !strings.Contains(output, "ghcrctl version") {
		t.Errorf("Expected root command output to contain version, got:\n%s", output)
	}
}

func TestRootCommandHasQuietFlag(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()

	// Check that --quiet flag exists
	quietFlag := cmd.PersistentFlags().Lookup("quiet")
	if quietFlag == nil {
		t.Error("Expected --quiet persistent flag to exist")
	}

	// Check shorthand
	qFlag := cmd.PersistentFlags().ShorthandLookup("q")
	if qFlag == nil {
		t.Error("Expected -q shorthand for --quiet flag")
	}
}
