package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompletionCommandExists(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()

	// Find the completion command
	completionCmd, _, err := cmd.Find([]string{"completion"})
	require.NoError(t, err, "Failed to find completion command")

	assert.Equal(t, "completion [bash|zsh|fish|powershell]", completionCmd.Use)
	assert.NotEmpty(t, completionCmd.Short, "Short description should not be empty")
}

func TestCompletionCommandGeneratesBash(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"completion", "bash"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.NoError(t, err, "completion bash failed")

	output := stdout.String()
	// Bash completion scripts contain these markers
	assert.Contains(t, output, "bash completion", "Expected bash completion script output")
	assert.Contains(t, output, "__start_ghcrctl", "Expected bash completion script output")
}

func TestCompletionCommandGeneratesZsh(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"completion", "zsh"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.NoError(t, err, "completion zsh failed")

	output := stdout.String()
	// Zsh completion scripts contain these markers
	assert.Contains(t, output, "zsh completion", "Expected zsh completion script output")
	assert.Contains(t, output, "compdef", "Expected zsh completion script output")
}

func TestCompletionCommandGeneratesFish(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"completion", "fish"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.NoError(t, err, "completion fish failed")

	output := stdout.String()
	// Fish completion scripts contain these markers
	assert.Contains(t, output, "fish completion", "Expected fish completion script output")
	assert.Contains(t, output, "complete -c ghcrctl", "Expected fish completion script output")
}

func TestCompletionCommandGeneratesPowershell(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"completion", "powershell"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.NoError(t, err, "completion powershell failed")

	output := stdout.String()
	// PowerShell completion scripts contain these markers
	assert.Contains(t, output, "powershell completion", "Expected powershell completion script output")
	assert.Contains(t, output, "Register-ArgumentCompleter", "Expected powershell completion script output")
}

func TestCompletionCommandInvalidShell(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"completion", "invalid"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	assert.Error(t, err, "Expected error for invalid shell")
}

func TestCompletionCommandNoArgs(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"completion"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	assert.Error(t, err, "Expected error when no shell specified")
}

// TestCompleteImageRefStructure tests that image reference completion function exists
func TestCompleteImageRefStructure(t *testing.T) {
	t.Parallel()

	// Test completeImageRef helper function exists and handles edge cases
	tests := []struct {
		name        string
		toComplete  string
		expectEmpty bool
	}{
		{
			name:        "empty input returns empty",
			toComplete:  "",
			expectEmpty: true,
		},
		{
			name:        "no slash returns empty (need owner/image format)",
			toComplete:  "myimage",
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Without a token, completion should return empty gracefully
			completions := completeImageRef(nil, tt.toComplete)
			if tt.expectEmpty {
				assert.Empty(t, completions, "Expected empty completions for %q", tt.toComplete)
			}
		})
	}
}

// TestVersionsCommandHasValidArgsFunction tests that versions command has completion
func TestVersionsCommandHasValidArgsFunction(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	versionsCmd, _, err := cmd.Find([]string{"list", "versions"})
	require.NoError(t, err, "Failed to find list versions command")

	assert.NotNil(t, versionsCmd.ValidArgsFunction, "Expected list versions command to have ValidArgsFunction for dynamic completion")
}

// TestLabelsCommandHasValidArgsFunction tests that labels command has completion
func TestLabelsCommandHasValidArgsFunction(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	labelsCmd, _, err := cmd.Find([]string{"get", "labels"})
	require.NoError(t, err, "Failed to find get labels command")

	assert.NotNil(t, labelsCmd.ValidArgsFunction, "Expected get labels command to have ValidArgsFunction for dynamic completion")
}

// TestSBOMCommandHasValidArgsFunction tests that sbom command has completion
func TestSBOMCommandHasValidArgsFunction(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	sbomCmd, _, err := cmd.Find([]string{"get", "sbom"})
	require.NoError(t, err, "Failed to find get sbom command")

	assert.NotNil(t, sbomCmd.ValidArgsFunction, "Expected get sbom command to have ValidArgsFunction for dynamic completion")
}

// TestProvenanceCommandHasValidArgsFunction tests that provenance command has completion
func TestProvenanceCommandHasValidArgsFunction(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	provenanceCmd, _, err := cmd.Find([]string{"get", "provenance"})
	require.NoError(t, err, "Failed to find get provenance command")

	assert.NotNil(t, provenanceCmd.ValidArgsFunction, "Expected get provenance command to have ValidArgsFunction for dynamic completion")
}
