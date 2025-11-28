package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/display"
)

func TestImagesCommandStructure(t *testing.T) {
	t.Parallel()
	// Verify images command exists and is properly structured
	cmd := NewRootCmd()
	imagesCmd, _, err := cmd.Find([]string{"images"})
	if err != nil {
		t.Fatalf("Failed to find images command: %v", err)
	}
	if imagesCmd == nil {
		t.Fatal("imagesCmd should not be nil")
	}

	if imagesCmd.Use != "images <owner>" {
		t.Errorf("Expected Use to be 'images <owner>', got '%s'", imagesCmd.Use)
	}

	if imagesCmd.RunE == nil {
		t.Error("imagesCmd should have RunE function")
	}
}

func TestImagesCommandArguments(t *testing.T) {
	t.Parallel()
	// Test that images command requires exactly one argument (owner)

	tests := []struct {
		name        string
		args        []string
		wantError   bool
		errContains string
	}{
		{
			name:        "no arguments",
			args:        []string{"images"},
			wantError:   true,
			errContains: "accepts 1 arg",
		},
		{
			name:        "with owner argument",
			args:        []string{"images", "mkoepf"},
			wantError:   false, // Will fail for other reasons (no token), but not arg validation
			errContains: "",
		},
		{
			name:        "with too many arguments",
			args:        []string{"images", "mkoepf", "extra"},
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

func TestImagesCommandHasJSONFlag(t *testing.T) {
	t.Parallel()
	// Verify that the images command has a --json flag
	cmd := NewRootCmd()
	imagesCmd, _, _ := cmd.Find([]string{"images"})
	jsonFlag := imagesCmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Error("images command should have --json flag")
	}
}

func TestOutputJSON(t *testing.T) {
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

func TestOutputTable(t *testing.T) {
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
			err := outputImagesTable(buf, tt.packages, tt.owner)

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
					if !strings.Contains(output, "No container images found") {
						t.Error("Expected 'No container images found' message for empty list")
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
