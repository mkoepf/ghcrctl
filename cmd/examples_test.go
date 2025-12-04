package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommandExamples ensures all commands have examples in their help text
func TestCommandExamples(t *testing.T) {
	tests := []struct {
		name        string
		commandName string
		wantExample bool
	}{
		// Commands that should have examples (updated for new structure)
		{"list packages command", "list packages", true},
		{"list versions command", "list versions", true},
		{"list images command", "list images", true},
		{"get labels command", "get labels", true},
		{"get sbom command", "get sbom", true},
		{"get provenance command", "get provenance", true},
		{"tag command", "tag", true},
		{"delete version command", "delete version", true},
		{"delete image command", "delete image", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := findCommand(rootCmd, tt.commandName)
			require.NotNil(t, cmd, "command %q not found", tt.commandName)

			hasExamples := strings.Contains(cmd.Long, "Examples:")
			if tt.wantExample {
				assert.True(t, hasExamples, "command %q is missing 'Examples:' section in Long description", tt.commandName)
			}
		})
	}
}

// TestExampleFormat ensures examples follow a consistent format
func TestExampleFormat(t *testing.T) {
	tests := []struct {
		name        string
		commandName string
	}{
		{"list packages command", "list packages"},
		{"list versions command", "list versions"},
		{"list images command", "list images"},
		{"get labels command", "get labels"},
		{"get sbom command", "get sbom"},
		{"get provenance command", "get provenance"},
		{"tag command", "tag"},
		{"delete version command", "delete version"},
		{"delete image command", "delete image"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := findCommand(rootCmd, tt.commandName)
			require.NotNil(t, cmd, "command %q not found", tt.commandName)

			// Check if Examples section exists
			if !strings.Contains(cmd.Long, "Examples:") {
				t.Skip("command has no Examples section yet")
			}

			// Verify examples start with "  # " (comment format)
			lines := strings.Split(cmd.Long, "\n")
			inExamples := false
			hasExampleComment := false

			for _, line := range lines {
				if strings.Contains(line, "Examples:") {
					inExamples = true
					continue
				}

				if inExamples && strings.TrimSpace(line) != "" {
					// Check if line is a comment or a command
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "# ") {
						hasExampleComment = true
					} else if strings.HasPrefix(trimmed, "ghcrctl ") {
						// This is OK - it's an example command
						continue
					}
				}
			}

			assert.True(t, !inExamples || hasExampleComment, "command %q has Examples section but no example comments starting with '# '", tt.commandName)
		})
	}
}

// findCommand searches for a command by name (supports subcommands like "delete version")
func findCommand(root *cobra.Command, name string) *cobra.Command {
	parts := strings.Split(name, " ")
	current := root

	for _, part := range parts {
		found := false
		for _, cmd := range current.Commands() {
			if cmd.Name() == part {
				current = cmd
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}

	return current
}
