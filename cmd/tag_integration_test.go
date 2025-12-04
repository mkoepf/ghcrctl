//go:build !mutating

package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTagCommandIntegration(t *testing.T) {
	t.Parallel()
	require.NotEmpty(t, os.Getenv("GITHUB_TOKEN"), "GITHUB_TOKEN not set")

	// Note: Actual tag creation tests are in tag_mutating_test.go (with //go:build mutating)

	tests := []struct {
		name          string
		args          []string
		wantError     bool
		errorContains string
	}{
		{
			name:          "missing arguments",
			args:          []string{"tag", "mkoepf/myimage"},
			wantError:     true,
			errorContains: "accepts 2 arg",
		},
		{
			name:          "too many arguments",
			args:          []string{"tag", "mkoepf/myimage", "v2.0", "extra"},
			wantError:     true,
			errorContains: "accepts 2 arg",
		},
		{
			name:          "missing selector",
			args:          []string{"tag", "mkoepf/ghcrctl-test-no-sbom", "new-tag"},
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
				require.Error(t, err, "Expected error but got none")
				if tt.errorContains != "" {
					assert.ErrorContains(t, err, tt.errorContains)
				}
			} else {
				assert.NoError(t, err, "Unexpected error: %s", errBuf.String())
			}
		})
	}
}

// TestTag_SourceTagNotFound tests error when source tag doesn't exist
func TestTag_SourceTagNotFound(t *testing.T) {
	t.Parallel()
	require.NotEmpty(t, os.Getenv("GITHUB_TOKEN"), "GITHUB_TOKEN not set")

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"tag", "mkoepf/ghcrctl-test-no-sbom", "new-tag", "--tag", "nonexistent-tag-12345"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.Error(t, err, "Expected error for nonexistent source tag, got none")

	// Should mention tag resolution failure
	assert.ErrorContains(t, err, "failed to resolve source tag")

	// Should be operational error, not show usage
	assert.NotContains(t, stderr.String(), "Usage:", "Operational error should not show usage hint")
}

// TestTag_InvalidPackage tests error for nonexistent package
func TestTag_InvalidPackage(t *testing.T) {
	t.Parallel()
	require.NotEmpty(t, os.Getenv("GITHUB_TOKEN"), "GITHUB_TOKEN not set")

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"tag", "mkoepf/nonexistent-package-12345", "new-tag", "--tag", "v1.0"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.Error(t, err, "Expected error for nonexistent package, got none")

	// Should be operational error, not show usage
	assert.NotContains(t, stderr.String(), "Usage:", "Operational error should not show usage hint")
}

// TestTagCommand_Help tests that tag command shows help
func TestTagCommand_Help(t *testing.T) {
	t.Parallel()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"tag", "--help"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	require.NoError(t, err, "tag --help failed")

	output := stdout.String()

	// Should show usage with arguments
	assert.Contains(t, output, "<owner/package>", "Expected help to show <owner/package> argument")

	// Should show flag descriptions
	assert.Contains(t, output, "Source version by tag", "Expected help to contain 'Source version by tag'")
}
