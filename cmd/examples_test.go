package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestCommandExamples ensures all commands have examples in their help text
func TestCommandExamples(t *testing.T) {
	tests := []struct {
		name        string
		commandName string
		wantExample bool
	}{
		// Commands that should have examples
		{"images command", "images", true},
		{"sbom command", "sbom", true},
		{"provenance command", "provenance", true},
		{"labels command", "labels", true},
		{"tag command", "tag", true},
		{"versions command", "versions", true},
		{"delete version command", "delete version", true},
		{"delete graph command", "delete graph", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := findCommand(rootCmd, tt.commandName)
			if cmd == nil {
				t.Fatalf("command %q not found", tt.commandName)
			}

			hasExamples := strings.Contains(cmd.Long, "Examples:")
			if tt.wantExample && !hasExamples {
				t.Errorf("command %q is missing 'Examples:' section in Long description", tt.commandName)
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
		{"images command", "images"},
		{"sbom command", "sbom"},
		{"provenance command", "provenance"},
		{"labels command", "labels"},
		{"tag command", "tag"},
		{"versions command", "versions"},
		{"delete version command", "delete version"},
		{"delete graph command", "delete graph"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := findCommand(rootCmd, tt.commandName)
			if cmd == nil {
				t.Fatalf("command %q not found", tt.commandName)
			}

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

			if inExamples && !hasExampleComment {
				t.Errorf("command %q has Examples section but no example comments starting with '# '", tt.commandName)
			}
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
