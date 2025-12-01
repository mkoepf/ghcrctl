package cmd

import (
	"strings"
	"testing"
)

func TestTagAddCommandStructure(t *testing.T) {
	t.Parallel()
	// Verify tag add command exists and is properly structured
	cmd := NewRootCmd()
	tagAddCmd, _, err := cmd.Find([]string{"tag", "add"})
	if err != nil {
		t.Fatalf("Failed to find tag add command: %v", err)
	}
	if tagAddCmd == nil {
		t.Fatal("tagAddCmd should not be nil")
	}

	if tagAddCmd.Use != "add <owner/package> <new-tag>" {
		t.Errorf("Expected Use to be 'add <owner/package> <new-tag>', got '%s'", tagAddCmd.Use)
	}

	if tagAddCmd.RunE == nil {
		t.Error("tagAddCmd should have RunE function")
	}

	if tagAddCmd.Short == "" {
		t.Error("tagAddCmd should have a Short description")
	}
}

func TestTagAddCommandArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		args      []string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "missing all arguments",
			args:      []string{"tag", "add"},
			wantError: true,
			errorMsg:  "accepts 2 arg",
		},
		{
			name:      "missing new-tag argument",
			args:      []string{"tag", "add", "mkoepf/myimage"},
			wantError: true,
			errorMsg:  "accepts 2 arg",
		},
		{
			name:      "too many arguments",
			args:      []string{"tag", "add", "mkoepf/myimage", "v2.0", "extra"},
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

func TestTagAddCommandHasFlags(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	tagAddCmd, _, err := cmd.Find([]string{"tag", "add"})
	if err != nil {
		t.Fatalf("Failed to find tag add command: %v", err)
	}

	// Check for selector flags
	flags := []string{"tag", "digest"}
	for _, flagName := range flags {
		flag := tagAddCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Expected --%s flag to exist", flagName)
		}
	}
}
