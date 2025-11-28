package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionCommandExists(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()

	// Find the completion command
	completionCmd, _, err := cmd.Find([]string{"completion"})
	if err != nil {
		t.Fatalf("Failed to find completion command: %v", err)
	}

	if completionCmd.Use != "completion [bash|zsh|fish|powershell]" {
		t.Errorf("Expected Use 'completion [bash|zsh|fish|powershell]', got '%s'", completionCmd.Use)
	}

	if completionCmd.Short == "" {
		t.Error("Short description should not be empty")
	}
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
	if err != nil {
		t.Fatalf("completion bash failed: %v", err)
	}

	output := stdout.String()
	// Bash completion scripts contain these markers
	if !strings.Contains(output, "bash completion") || !strings.Contains(output, "__start_ghcrctl") {
		t.Error("Expected bash completion script output")
	}
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
	if err != nil {
		t.Fatalf("completion zsh failed: %v", err)
	}

	output := stdout.String()
	// Zsh completion scripts contain these markers
	if !strings.Contains(output, "zsh completion") || !strings.Contains(output, "compdef") {
		t.Error("Expected zsh completion script output")
	}
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
	if err != nil {
		t.Fatalf("completion fish failed: %v", err)
	}

	output := stdout.String()
	// Fish completion scripts contain these markers
	if !strings.Contains(output, "fish completion") || !strings.Contains(output, "complete -c ghcrctl") {
		t.Error("Expected fish completion script output")
	}
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
	if err != nil {
		t.Fatalf("completion powershell failed: %v", err)
	}

	output := stdout.String()
	// PowerShell completion scripts contain these markers
	if !strings.Contains(output, "powershell completion") || !strings.Contains(output, "Register-ArgumentCompleter") {
		t.Error("Expected powershell completion script output")
	}
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
	if err == nil {
		t.Error("Expected error for invalid shell")
	}
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
	if err == nil {
		t.Error("Expected error when no shell specified")
	}
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
			if tt.expectEmpty && len(completions) != 0 {
				t.Errorf("Expected empty completions for %q, got %d", tt.toComplete, len(completions))
			}
		})
	}
}

// TestVersionsCommandHasValidArgsFunction tests that versions command has completion
func TestVersionsCommandHasValidArgsFunction(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	versionsCmd, _, err := cmd.Find([]string{"versions"})
	if err != nil {
		t.Fatalf("Failed to find versions command: %v", err)
	}

	if versionsCmd.ValidArgsFunction == nil {
		t.Error("Expected versions command to have ValidArgsFunction for dynamic completion")
	}
}

// TestLabelsCommandHasValidArgsFunction tests that labels command has completion
func TestLabelsCommandHasValidArgsFunction(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	labelsCmd, _, err := cmd.Find([]string{"labels"})
	if err != nil {
		t.Fatalf("Failed to find labels command: %v", err)
	}

	if labelsCmd.ValidArgsFunction == nil {
		t.Error("Expected labels command to have ValidArgsFunction for dynamic completion")
	}
}

// TestSBOMCommandHasValidArgsFunction tests that sbom command has completion
func TestSBOMCommandHasValidArgsFunction(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	sbomCmd, _, err := cmd.Find([]string{"sbom"})
	if err != nil {
		t.Fatalf("Failed to find sbom command: %v", err)
	}

	if sbomCmd.ValidArgsFunction == nil {
		t.Error("Expected sbom command to have ValidArgsFunction for dynamic completion")
	}
}

// TestProvenanceCommandHasValidArgsFunction tests that provenance command has completion
func TestProvenanceCommandHasValidArgsFunction(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	provenanceCmd, _, err := cmd.Find([]string{"provenance"})
	if err != nil {
		t.Fatalf("Failed to find provenance command: %v", err)
	}

	if provenanceCmd.ValidArgsFunction == nil {
		t.Error("Expected provenance command to have ValidArgsFunction for dynamic completion")
	}
}
