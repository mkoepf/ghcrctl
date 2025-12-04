package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOutputFormatFlag ensures all commands that support --json also support -o json
func TestOutputFormatFlag(t *testing.T) {
	// Updated command paths for new structure
	commandsWithJSON := []string{
		"list packages",
		"list versions",
		"list images",
		"get labels",
		"get sbom",
		"get provenance",
	}

	for _, cmdName := range commandsWithJSON {
		t.Run(cmdName+" supports -o flag", func(t *testing.T) {
			cmd := findCommand(rootCmd, cmdName)
			require.NotNil(t, cmd, "command %q not found", cmdName)

			// Check if -o flag exists
			outputFlag := cmd.Flags().Lookup("output")
			if assert.NotNil(t, outputFlag, "command %q does not have -o/--output flag", cmdName) {
				// Check shorthand
				assert.Equal(t, "o", outputFlag.Shorthand, "command %q --output flag does not have shorthand -o", cmdName)
			}

			// Ensure --json flag still exists for backward compatibility
			jsonFlag := cmd.Flags().Lookup("json")
			assert.NotNil(t, jsonFlag, "command %q does not have --json flag (backward compatibility)", cmdName)
		})
	}
}

// TestOutputFormatValues tests that -o only accepts valid values
func TestOutputFormatValues(t *testing.T) {
	tests := []struct {
		command     string
		outputValue string
		shouldError bool
	}{
		{"list packages", "json", false},
		{"list packages", "table", false},
		{"list packages", "yaml", true}, // not supported
		{"list packages", "csv", true},  // not supported
		{"get labels", "json", false},
		{"get labels", "table", false},
		{"get sbom", "json", false},
		{"get sbom", "table", false},
		{"get provenance", "json", false},
		{"get provenance", "table", false},
		{"list versions", "json", false},
		{"list versions", "table", false},
	}

	for _, tt := range tests {
		t.Run(tt.command+" -o "+tt.outputValue, func(t *testing.T) {
			cmd := findCommand(rootCmd, tt.command)
			require.NotNil(t, cmd, "command %q not found", tt.command)

			outputFlag := cmd.Flags().Lookup("output")
			if outputFlag == nil {
				t.Skip("command does not have -o flag yet")
			}

			// Check if the flag has validation (this will be implemented)
			// For now, we just verify the flag exists
		})
	}
}

// TestBackwardCompatibility ensures --json still works after adding -o
func TestBackwardCompatibility(t *testing.T) {
	commandsWithJSON := []string{
		"list packages",
		"list versions",
		"get labels",
		"get sbom",
		"get provenance",
	}

	for _, cmdName := range commandsWithJSON {
		t.Run(cmdName+" --json still works", func(t *testing.T) {
			cmd := findCommand(rootCmd, cmdName)
			require.NotNil(t, cmd, "command %q not found", cmdName)

			// Ensure --json flag exists
			jsonFlag := cmd.Flags().Lookup("json")
			assert.NotNil(t, jsonFlag, "command %q lost --json flag (backward compatibility broken)", cmdName)
		})
	}
}

// TestOutputFlagDescription checks that help text is clear
func TestOutputFlagDescription(t *testing.T) {
	commandsWithJSON := []string{
		"list packages",
		"list versions",
		"get labels",
		"get sbom",
		"get provenance",
	}

	for _, cmdName := range commandsWithJSON {
		t.Run(cmdName+" has clear -o description", func(t *testing.T) {
			cmd := findCommand(rootCmd, cmdName)
			require.NotNil(t, cmd, "command %q not found", cmdName)

			outputFlag := cmd.Flags().Lookup("output")
			if outputFlag == nil {
				t.Skip("command does not have -o flag yet")
			}

			// Check that description mentions valid formats
			usage := outputFlag.Usage
			assert.Contains(t, usage, "json", "command %q -o flag usage should mention 'json' format", cmdName)
		})
	}
}
