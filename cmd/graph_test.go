package cmd

import (
	"strings"
	"testing"
)

func TestGraphCommandStructure(t *testing.T) {
	// Verify graph command exists and is properly structured
	if graphCmd == nil {
		t.Fatal("graphCmd should not be nil")
	}

	if graphCmd.Use != "graph <image>" {
		t.Errorf("Expected Use to be 'graph <image>', got '%s'", graphCmd.Use)
	}

	if graphCmd.RunE == nil {
		t.Error("graphCmd should have RunE function")
	}
}

func TestGraphCommandArguments(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "missing image argument",
			args:      []string{"graph"},
			wantError: true,
			errorMsg:  "accepts 1 arg",
		},
		{
			name:      "too many arguments",
			args:      []string{"graph", "image1", "image2"},
			wantError: true,
			errorMsg:  "accepts 1 arg",
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

func TestGraphCommandHasFlags(t *testing.T) {
	// Verify that the graph command has expected flags
	tagFlag := graphCmd.Flags().Lookup("tag")
	if tagFlag == nil {
		t.Error("graph command should have --tag flag")
	}

	jsonFlag := graphCmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Error("graph command should have --json flag")
	}
}
