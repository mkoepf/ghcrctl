package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/display"
)

func TestListPackagesCommandStructure(t *testing.T) {
	t.Parallel()
	// Verify list packages command exists and is properly structured
	cmd := NewRootCmd()
	packagesCmd, _, err := cmd.Find([]string{"list", "packages"})
	if err != nil {
		t.Fatalf("Failed to find list packages command: %v", err)
	}
	if packagesCmd == nil {
		t.Fatal("packagesCmd should not be nil")
	}

	if packagesCmd.Use != "packages <owner>" {
		t.Errorf("Expected Use to be 'packages <owner>', got '%s'", packagesCmd.Use)
	}

	if packagesCmd.RunE == nil {
		t.Error("packagesCmd should have RunE function")
	}
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
				if err == nil {
					t.Error("Expected error")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing %q, got: %v", tt.errContains, err)
				}
			} else {
				// For valid args, it may still fail but not due to argument validation
				if err != nil && strings.Contains(err.Error(), "accepts") {
					t.Errorf("Should not fail on argument count: %v", err)
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
	if jsonFlag == nil {
		t.Error("list packages command should have --json flag")
	}
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
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Verify JSON is valid
				output := buf.String()
				var parsed []string
				if err := json.Unmarshal([]byte(output), &parsed); err != nil {
					t.Errorf("Output is not valid JSON: %v", err)
				}

				// Verify contents match
				if len(parsed) != len(tt.packages) {
					t.Errorf("Expected %d packages, got %d", len(tt.packages), len(parsed))
				}
				for i, pkg := range tt.packages {
					if i >= len(parsed) {
						break
					}
					if parsed[i] != pkg {
						t.Errorf("Package %d: expected %s, got %s", i, pkg, parsed[i])
					}
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
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Verify output contains expected elements
				output := buf.String()

				// Should always mention the owner
				if !strings.Contains(output, tt.owner) {
					t.Errorf("Expected output to contain owner %s", tt.owner)
				}

				// Verify each package appears in output
				for _, pkg := range tt.packages {
					if !strings.Contains(output, pkg) {
						t.Errorf("Expected output to contain package %s", pkg)
					}
				}

				// Verify correct message for empty list
				if len(tt.packages) == 0 {
					if !strings.Contains(output, "No packages found") {
						t.Error("Expected 'No packages found' message for empty list")
					}
				} else {
					// Non-empty list should show count
					if !strings.Contains(output, "Total:") {
						t.Error("Expected 'Total:' in output for non-empty list")
					}
				}
			}
		})
	}
}
