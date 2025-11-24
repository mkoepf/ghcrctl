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
		name          string
		args          []string
		wantError     bool
		errorContains string
	}{
		{
			name:          "missing arguments",
			args:          []string{"tag", "myimage"},
			wantError:     true,
			errorContains: "accepts 3 arg",
		},
		{
			name:          "too many arguments",
			args:          []string{"tag", "myimage", "v1.0", "v2.0", "extra"},
			wantError:     true,
			errorContains: "accepts 3 arg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
