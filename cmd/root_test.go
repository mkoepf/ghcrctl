package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommand(t *testing.T) {
	t.Parallel()
	// Test root command structure
	cmd := NewRootCmd()

	assert.Equal(t, "ghcrctl", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify Long description contains expected functionality
	expectedKeywords := []string{
		"packages",
		"OCI",
		"metadata",
		"deletion",
	}

	for _, keyword := range expectedKeywords {
		assert.Contains(t, cmd.Long, keyword)
	}
}

func TestRootCommandHasListSubcommand(t *testing.T) {
	t.Parallel()
	// Verify list subcommand is registered (contains packages, versions, images)
	cmd := NewRootCmd()
	listCmd, _, err := cmd.Find([]string{"list"})
	require.NoError(t, err, "Failed to find list subcommand")

	// Verify list has packages subcommand
	packagesFound := false
	for _, c := range listCmd.Commands() {
		if strings.HasPrefix(c.Use, "packages") {
			packagesFound = true
			break
		}
	}

	assert.True(t, packagesFound, "Expected packages subcommand under list command")
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
	assert.NoError(t, err, "Expected --help to succeed")
}

func TestRootCommandVersion(t *testing.T) {
	t.Parallel()
	// Test that --version flag is available and returns version info
	cmd := NewRootCmd()

	assert.NotEmpty(t, cmd.Version, "cmd.Version should not be empty")

	// Version should be set (either "dev" or injected at build time)
	validVersion := cmd.Version == "dev" || strings.HasPrefix(cmd.Version, "v")
	assert.True(t, validVersion, "Expected version to be 'dev' or start with 'v', got %q", cmd.Version)
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
	assert.Contains(t, output, "ghcrctl version", "Expected root command output to contain version")
}

func TestRootCommandHasQuietFlag(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()

	// Check that --quiet flag exists
	quietFlag := cmd.PersistentFlags().Lookup("quiet")
	assert.NotNil(t, quietFlag, "Expected --quiet persistent flag to exist")

	// Check shorthand
	qFlag := cmd.PersistentFlags().ShorthandLookup("q")
	assert.NotNil(t, qFlag, "Expected -q shorthand for --quiet flag")
}
