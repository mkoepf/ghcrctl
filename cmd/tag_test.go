package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestTagCommandStructure(t *testing.T) {
	t.Parallel()
	// Verify tag command exists and is properly structured
	cmd := NewRootCmd()
	tagCmd, _, err := cmd.Find([]string{"tag"})
	if err != nil {
		t.Fatalf("Failed to find tag command: %v", err)
	}
	if tagCmd == nil {
		t.Fatal("tagCmd should not be nil")
	}

	if tagCmd.Use != "tag <owner/package> <new-tag>" {
		t.Errorf("Expected Use to be 'tag <owner/package> <new-tag>', got '%s'", tagCmd.Use)
	}

	if tagCmd.RunE == nil {
		t.Error("tagCmd should have RunE function")
	}

	if tagCmd.Short == "" {
		t.Error("tagCmd should have a Short description")
	}
}

func TestTagCommandArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		args      []string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "missing all arguments",
			args:      []string{"tag"},
			wantError: true,
			errorMsg:  "accepts 2 arg",
		},
		{
			name:      "missing new-tag argument",
			args:      []string{"tag", "mkoepf/myimage"},
			wantError: true,
			errorMsg:  "accepts 2 arg",
		},
		{
			name:      "too many arguments",
			args:      []string{"tag", "mkoepf/myimage", "v2.0", "extra"},
			wantError: true,
			errorMsg:  "accepts 2 arg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCmd()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestTagCommandHasFlags(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	tagCmd, _, err := cmd.Find([]string{"tag"})
	if err != nil {
		t.Fatalf("Failed to find tag command: %v", err)
	}

	// Check for selector flags
	flags := []string{"tag", "digest", "version"}
	for _, flagName := range flags {
		flag := tagCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Expected --%s flag to exist", flagName)
		}
	}
}

// =============================================================================
// Tests for ExecuteTagAdd
// =============================================================================

// mockTagAdder implements TagAdder for testing
type mockTagAdder struct {
	resolvedDigest string
	resolveErr     error
	addErr         error
}

func (m *mockTagAdder) ResolveTag(ctx context.Context, fullImage, tag string) (string, error) {
	if m.resolveErr != nil {
		return "", m.resolveErr
	}
	return m.resolvedDigest, nil
}

func (m *mockTagAdder) AddTagByDigest(ctx context.Context, fullImage, digest, newTag string) error {
	return m.addErr
}

func TestExecuteTagAdd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		params         TagAddParams
		resolvedDigest string
		resolveErr     error
		addErr         error
		wantErr        bool
		wantErrMsg     string
		wantOutput     []string
	}{
		{
			name: "successful add with source tag",
			params: TagAddParams{
				Owner:       "testowner",
				PackageName: "testimage",
				NewTag:      "latest",
				SourceTag:   "v1.0.0",
			},
			resolvedDigest: "sha256:abc123",
			wantErr:        false,
			wantOutput: []string{
				"Successfully added tag 'latest' to testimage (source: v1.0.0)",
			},
		},
		{
			name: "successful add with source digest",
			params: TagAddParams{
				Owner:        "testowner",
				PackageName:  "testimage",
				NewTag:       "production",
				SourceDigest: "sha256:abc123def456789",
			},
			wantErr: false,
			wantOutput: []string{
				"Successfully added tag 'production' to testimage (source: sha256:abc123def456)",
			},
		},
		{
			name: "resolve tag failure",
			params: TagAddParams{
				Owner:       "testowner",
				PackageName: "testimage",
				NewTag:      "latest",
				SourceTag:   "nonexistent",
			},
			resolveErr: fmt.Errorf("tag not found"),
			wantErr:    true,
			wantErrMsg: "failed to resolve source tag 'nonexistent'",
		},
		{
			name: "add tag failure",
			params: TagAddParams{
				Owner:        "testowner",
				PackageName:  "testimage",
				NewTag:       "latest",
				SourceDigest: "sha256:abc123def456789",
			},
			addErr:     fmt.Errorf("permission denied"),
			wantErr:    true,
			wantErrMsg: "failed to add tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTagAdder{
				resolvedDigest: tt.resolvedDigest,
				resolveErr:     tt.resolveErr,
				addErr:         tt.addErr,
			}

			var buf bytes.Buffer
			ctx := context.Background()

			err := ExecuteTagAdd(ctx, mock, tt.params, &buf)

			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteTagAdd() error = %v, wantErr = %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.wantErrMsg != "" {
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("error message should contain %q, got %q", tt.wantErrMsg, err.Error())
				}
				return
			}

			output := buf.String()
			for _, want := range tt.wantOutput {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\nGot:\n%s", want, output)
				}
			}
		})
	}
}
