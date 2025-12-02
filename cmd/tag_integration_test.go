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
		t.Fatal("GITHUB_TOKEN not set")
	}

	// Note: Actual tag creation tests are in tag_mutating_test.go (with //go:build mutating)

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
		{
			name:          "missing selector",
			args:          []string{"tag", "add", "mkoepf/ghcrctl-test-no-sbom", "new-tag"},
			wantError:     true,
			errorContains: "selector required",
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

// TestTagAdd_SourceTagNotFound tests error when source tag doesn't exist
func TestTagAdd_SourceTagNotFound(t *testing.T) {
	t.Parallel()
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Fatal("GITHUB_TOKEN not set")
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"tag", "add", "mkoepf/ghcrctl-test-no-sbom", "new-tag", "--tag", "nonexistent-tag-12345"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for nonexistent source tag, got none")
	}

	// Should mention tag resolution failure
	if !strings.Contains(err.Error(), "failed to resolve source tag") {
		t.Errorf("Expected error about tag resolution, got: %v", err)
	}

	// Should be operational error, not show usage
	if strings.Contains(stderr.String(), "Usage:") {
		t.Error("Operational error should not show usage hint")
	}
}

// TestTagAdd_InvalidPackage tests error for nonexistent package
func TestTagAdd_InvalidPackage(t *testing.T) {
	t.Parallel()
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Fatal("GITHUB_TOKEN not set")
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"tag", "add", "mkoepf/nonexistent-package-12345", "new-tag", "--tag", "v1.0"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for nonexistent package, got none")
	}

	// Should be operational error, not show usage
	if strings.Contains(stderr.String(), "Usage:") {
		t.Error("Operational error should not show usage hint")
	}
}

// TestTagCommand_ParentHelp tests that tag command shows help with subcommands
func TestTagCommand_ParentHelp(t *testing.T) {
	t.Parallel()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"tag", "--help"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("tag --help failed: %v", err)
	}

	output := stdout.String()

	// Should show subcommand info
	if !strings.Contains(output, "add") {
		t.Error("Expected help to mention 'add' subcommand")
	}

	// Should show description
	if !strings.Contains(output, "Manage tags") {
		t.Error("Expected help to contain 'Manage tags'")
	}
}
