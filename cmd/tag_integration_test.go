package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestTagCommandIntegration(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("GITHUB_TOKEN not set, skipping integration test")
	}

	// Note: This test requires write:packages permission which the integration test token doesn't have
	// We verify the command structure and error handling, but can't test actual tag creation
	// without a token with write permissions

	tests := []struct {
		name           string
		args           []string
		wantError      bool
		errorContains  string
		skipReason     string
	}{
		{
			name:           "missing arguments",
			args:           []string{"tag", "myimage"},
			wantError:      true,
			errorContains:  "accepts 3 arg",
		},
		{
			name:           "too many arguments",
			args:           []string{"tag", "myimage", "v1.0", "v2.0", "extra"},
			wantError:      true,
			errorContains:  "accepts 3 arg",
		},
		{
			name:           "requires write permission",
			args:           []string{"tag", "ghcrctl-test-with-sbom", "latest", "test-tag"},
			wantError:      true,
			errorContains:  "permission",
			skipReason:     "Requires write:packages token which integration tests don't have",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipReason != "" {
				t.Skip(tt.skipReason)
			}

			rootCmd.SetArgs(tt.args)
			var outBuf, errBuf bytes.Buffer
			rootCmd.SetOut(&outBuf)
			rootCmd.SetErr(&errBuf)

			err := rootCmd.Execute()

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					t.Logf("Stderr: %s", errBuf.String())
				}
			}

			rootCmd.SetArgs([]string{})
		})
	}
}

// TestTagCommandWithWriteToken tests the actual tag creation functionality
// This test is skipped by default as it requires a token with write:packages permission
func TestTagCommandWithWriteToken(t *testing.T) {
	t.Skip("This test requires write:packages permission - run manually with appropriate token")

	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("GITHUB_TOKEN not set, skipping integration test")
	}

	// This test would verify:
	// 1. Tag command successfully creates a new tag
	// 2. New tag points to same digest as source tag
	// 3. Both tags are visible in subsequent operations

	// Example test structure (commented out):
	// args := []string{"tag", "ghcrctl-test-with-sbom", "latest", "integration-test-tag"}
	// rootCmd.SetArgs(args)
	// err := rootCmd.Execute()
	// if err != nil {
	//     t.Fatalf("Tag command failed: %v", err)
	// }
	//
	// Verify the new tag exists and points to same digest...
}
