//go:build !mutating

package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestTagCommandIntegration(t *testing.T) {
	t.Parallel()
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("GITHUB_TOKEN not set, skipping integration test")
	}

	// TODO: Add integration tests for actual tag creation (token has write:packages permission)

	tests := []struct {
		name          string
		args          []string
		wantError     bool
		errorContains string
	}{
		{
			name:          "missing arguments",
			args:          []string{"tag", "add", "mkoepf/myimage"},
			wantError:     true,
			errorContains: "accepts 2 arg",
		},
		{
			name:          "too many arguments",
			args:          []string{"tag", "add", "mkoepf/myimage", "v2.0", "extra"},
			wantError:     true,
			errorContains: "accepts 2 arg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh command instance for each test
			cmd := NewRootCmd()
			cmd.SetArgs(tt.args)
			var outBuf, errBuf bytes.Buffer
			cmd.SetOut(&outBuf)
			cmd.SetErr(&errBuf)

			err := cmd.Execute()

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
		})
	}
}
