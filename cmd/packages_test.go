package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/display"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListPackagesCommandStructure(t *testing.T) {
	t.Parallel()
	// Verify list packages command exists and is properly structured
	cmd := NewRootCmd()
	packagesCmd, _, err := cmd.Find([]string{"list", "packages"})
	require.NoError(t, err, "Failed to find list packages command")
	require.NotNil(t, packagesCmd, "packagesCmd should not be nil")

	assert.Equal(t, "packages <owner>", packagesCmd.Use)
	assert.NotNil(t, packagesCmd.RunE, "packagesCmd should have RunE function")
}

func TestListPackagesCommandArguments(t *testing.T) {
	t.Parallel()
	// Test that list packages command requires exactly one argument (owner)

	tests := []struct {
		name        string
		args        []string
		wantError   bool
		errContains string
	}{
		{
			name:        "no arguments",
			args:        []string{"list", "packages"},
			wantError:   true,
			errContains: "accepts 1 arg",
		},
		{
			name:        "with owner argument",
			args:        []string{"list", "packages", "mkoepf"},
			wantError:   false, // Will fail for other reasons (no token), but not arg validation
			errContains: "",
		},
		{
			name:        "with too many arguments",
			args:        []string{"list", "packages", "mkoepf", "extra"},
			wantError:   true,
			errContains: "accepts 1 arg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCmd()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if tt.wantError {
				require.Error(t, err, "Expected error")
				if tt.errContains != "" {
					assert.ErrorContains(t, err, tt.errContains)
				}
			} else {
				// For valid args, it may still fail but not due to argument validation
				if err != nil {
					assert.NotContains(t, err.Error(), "accepts", "Should not fail on argument count")
				}
			}
		})
	}
}

func TestListPackagesCommandHasJSONFlag(t *testing.T) {
	t.Parallel()
	// Verify that the list packages command has a --json flag
	cmd := NewRootCmd()
	packagesCmd, _, _ := cmd.Find([]string{"list", "packages"})
	jsonFlag := packagesCmd.Flags().Lookup("json")
	assert.NotNil(t, jsonFlag, "list packages command should have --json flag")
}

func TestPackagesOutputJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		packages []string
		wantErr  bool
	}{
		{
			name:     "empty list",
			packages: []string{},
			wantErr:  false,
		},
		{
			name:     "single package",
			packages: []string{"package1"},
			wantErr:  false,
		},
		{
			name:     "multiple packages",
			packages: []string{"alpha", "beta", "gamma"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture output in buffer
			buf := &bytes.Buffer{}
			err := display.OutputJSON(buf, tt.packages)

			if tt.wantErr {
				assert.Error(t, err, "Expected error but got none")
			} else {
				require.NoError(t, err, "Unexpected error")

				// Verify JSON is valid
				output := buf.String()
				var parsed []string
				err := json.Unmarshal([]byte(output), &parsed)
				require.NoError(t, err, "Output is not valid JSON")

				// Verify contents match
				assert.Len(t, parsed, len(tt.packages))
				for i, pkg := range tt.packages {
					if i >= len(parsed) {
						break
					}
					assert.Equal(t, pkg, parsed[i], "Package %d mismatch", i)
				}
			}
		})
	}
}

func TestPackagesOutputTable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		packages []string
		owner    string
		wantErr  bool
	}{
		{
			name:     "empty list",
			packages: []string{},
			owner:    "testowner",
			wantErr:  false,
		},
		{
			name:     "single package",
			packages: []string{"package1"},
			owner:    "testowner",
			wantErr:  false,
		},
		{
			name:     "multiple packages",
			packages: []string{"alpha", "beta", "gamma"},
			owner:    "testowner",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture output in buffer
			buf := &bytes.Buffer{}
			err := outputListPackagesTable(buf, tt.packages, tt.owner, false)

			if tt.wantErr {
				assert.Error(t, err, "Expected error but got none")
			} else {
				require.NoError(t, err, "Unexpected error")

				// Verify output contains expected elements
				output := buf.String()

				// Should always mention the owner
				assert.Contains(t, output, tt.owner, "Expected output to contain owner")

				// Verify each package appears in output
				for _, pkg := range tt.packages {
					assert.Contains(t, output, pkg, "Expected output to contain package")
				}

				// Verify correct message for empty list
				if len(tt.packages) == 0 {
					assert.Contains(t, output, "No packages found", "Expected 'No packages found' message for empty list")
				} else {
					// Non-empty list should show count
					assert.Contains(t, output, "Total:", "Expected 'Total:' in output for non-empty list")
				}
			}
		})
	}
}
