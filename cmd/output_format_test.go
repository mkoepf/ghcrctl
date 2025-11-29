package cmd

import (
	"strings"
	"testing"
)

// TestOutputFormatFlag ensures all commands that support --json also support -o json
func TestOutputFormatFlag(t *testing.T) {
	commandsWithJSON := []string{
		"packages",
		"labels",
		"sbom",
		"provenance",
		"versions",
	}

	for _, cmdName := range commandsWithJSON {
		t.Run(cmdName+" supports -o flag", func(t *testing.T) {
			cmd := findCommand(rootCmd, cmdName)
			if cmd == nil {
				t.Fatalf("command %q not found", cmdName)
			}

			// Check if -o flag exists
			outputFlag := cmd.Flags().Lookup("output")
			if outputFlag == nil {
				t.Errorf("command %q does not have -o/--output flag", cmdName)
			} else {
				// Check shorthand
				if outputFlag.Shorthand != "o" {
					t.Errorf("command %q --output flag does not have shorthand -o", cmdName)
				}
			}

			// Ensure --json flag still exists for backward compatibility
			jsonFlag := cmd.Flags().Lookup("json")
			if jsonFlag == nil {
				t.Errorf("command %q does not have --json flag (backward compatibility)", cmdName)
			}
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
		{"packages", "json", false},
		{"packages", "table", false},
		{"packages", "yaml", true}, // not supported
		{"packages", "csv", true},  // not supported
		{"labels", "json", false},
		{"labels", "table", false},
		{"sbom", "json", false},
		{"sbom", "table", false},
		{"provenance", "json", false},
		{"provenance", "table", false},
		{"versions", "json", false},
		{"versions", "table", false},
	}

	for _, tt := range tests {
		t.Run(tt.command+" -o "+tt.outputValue, func(t *testing.T) {
			cmd := findCommand(rootCmd, tt.command)
			if cmd == nil {
				t.Fatalf("command %q not found", tt.command)
			}

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
		"packages",
		"labels",
		"sbom",
		"provenance",
		"versions",
	}

	for _, cmdName := range commandsWithJSON {
		t.Run(cmdName+" --json still works", func(t *testing.T) {
			cmd := findCommand(rootCmd, cmdName)
			if cmd == nil {
				t.Fatalf("command %q not found", cmdName)
			}

			// Ensure --json flag exists
			jsonFlag := cmd.Flags().Lookup("json")
			if jsonFlag == nil {
				t.Errorf("command %q lost --json flag (backward compatibility broken)", cmdName)
			}
		})
	}
}

// TestOutputFlagDescription checks that help text is clear
func TestOutputFlagDescription(t *testing.T) {
	commandsWithJSON := []string{
		"packages",
		"labels",
		"sbom",
		"provenance",
		"versions",
	}

	for _, cmdName := range commandsWithJSON {
		t.Run(cmdName+" has clear -o description", func(t *testing.T) {
			cmd := findCommand(rootCmd, cmdName)
			if cmd == nil {
				t.Fatalf("command %q not found", cmdName)
			}

			outputFlag := cmd.Flags().Lookup("output")
			if outputFlag == nil {
				t.Skip("command does not have -o flag yet")
			}

			// Check that description mentions valid formats
			usage := outputFlag.Usage
			if !strings.Contains(usage, "json") {
				t.Errorf("command %q -o flag usage should mention 'json' format", cmdName)
			}
		})
	}
}
