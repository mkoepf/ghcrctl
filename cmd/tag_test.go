package cmd

import (
	"strings"
	"testing"
)

func TestTagCommandStructure(t *testing.T) {
	// Verify tag command exists and is properly structured
	if tagCmd == nil {
		t.Fatal("tagCmd should not be nil")
	}

	if tagCmd.Use != "tag <image> <existing-tag> <new-tag>" {
		t.Errorf("Expected Use to be 'tag <image> <existing-tag> <new-tag>', got '%s'", tagCmd.Use)
	}

	if tagCmd.RunE == nil {
		t.Error("tagCmd should have RunE function")
	}

	if tagCmd.Short == "" {
		t.Error("tagCmd should have a Short description")
	}
}

func TestTagCommandArguments(t *testing.T) {
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
			errorMsg:  "accepts 3 arg",
		},
		{
			name:      "missing new-tag argument",
			args:      []string{"tag", "myimage", "v1.0"},
			wantError: true,
			errorMsg:  "accepts 3 arg",
		},
		{
			name:      "too many arguments",
			args:      []string{"tag", "myimage", "v1.0", "v2.0", "extra"},
			wantError: true,
			errorMsg:  "accepts 3 arg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

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

			rootCmd.SetArgs([]string{})
		})
	}
}
