package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestImagesCommandStructure(t *testing.T) {
	// Verify images command exists and is properly structured
	if imagesCmd == nil {
		t.Fatal("imagesCmd should not be nil")
	}

	if imagesCmd.Use != "images" {
		t.Errorf("Expected Use to be 'images', got '%s'", imagesCmd.Use)
	}

	if imagesCmd.RunE == nil {
		t.Error("imagesCmd should have RunE function")
	}
}

func TestImagesCommandWithoutConfig(t *testing.T) {
	// Test that images command fails when owner not configured

	// Create a temp config directory with no config
	tempDir, err := os.MkdirTemp("", "ghcrctl-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save original HOME and USERPROFILE (for Windows)
	originalHome := os.Getenv("HOME")
	originalUserProfile := os.Getenv("USERPROFILE")
	defer func() {
		os.Setenv("HOME", originalHome)
		if originalUserProfile != "" {
			os.Setenv("USERPROFILE", originalUserProfile)
		} else {
			os.Unsetenv("USERPROFILE")
		}
	}()

	// Set HOME and USERPROFILE to temp dir (no config)
	os.Setenv("HOME", tempDir)
	os.Setenv("USERPROFILE", tempDir)

	// Execute images command - should fail with helpful error
	rootCmd.SetArgs([]string{"images"})
	err = rootCmd.Execute()

	if err == nil {
		t.Error("Expected error when owner not configured, got none")
	}

	// Error should mention configuration
	if !strings.Contains(err.Error(), "owner") && !strings.Contains(err.Error(), "config") {
		t.Errorf("Expected error about configuration, got: %v", err)
	}

	rootCmd.SetArgs([]string{})
}

func TestImagesCommandArguments(t *testing.T) {
	// Test that images command accepts no arguments

	tests := []struct {
		name      string
		args      []string
		wantError bool
	}{
		{
			name:      "no arguments (valid)",
			args:      []string{"images"},
			wantError: false, // Will fail for other reasons (no config), but not arg validation
		},
		{
			name:      "with unexpected argument",
			args:      []string{"images", "unexpected"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			// For "no arguments" test, we expect it to fail but not due to args
			if tt.name == "no arguments (valid)" {
				// Just check it doesn't complain about arguments
				if err != nil && strings.Contains(err.Error(), "accepts") {
					t.Errorf("Should not fail on argument count: %v", err)
				}
			} else if tt.wantError {
				if err == nil {
					t.Error("Expected error for extra arguments")
				}
			}

			rootCmd.SetArgs([]string{})
		})
	}
}

func TestImagesCommandHasJSONFlag(t *testing.T) {
	// Verify that the images command has a --json flag
	jsonFlag := imagesCmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Error("images command should have --json flag")
	}
}

func TestOutputJSON(t *testing.T) {
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
			err := outputJSON(buf, tt.packages)

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
			err := outputTable(buf, tt.packages, tt.owner)

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
