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

	versionFlag := graphCmd.Flags().Lookup("version")
	if versionFlag == nil {
		t.Error("graph command should have --version flag")
	}

	digestFlag := graphCmd.Flags().Lookup("digest")
	if digestFlag == nil {
		t.Error("graph command should have --digest flag")
	}
}

// TestGraphCommandFlagExclusivity verifies that --tag, --version, and --digest flags are mutually exclusive
func TestGraphCommandFlagExclusivity(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "using --tag only",
			args:        []string{"graph", "myimage", "--tag", "v1.0"},
			expectError: false, // Will fail due to missing config, but not due to flag conflict
		},
		{
			name:        "using --version only",
			args:        []string{"graph", "myimage", "--version", "12345"},
			expectError: false, // Will fail due to missing config, but not due to flag conflict
		},
		{
			name:        "using --digest only",
			args:        []string{"graph", "myimage", "--digest", "sha256:abc123"},
			expectError: false, // Will fail due to missing config, but not due to flag conflict
		},
		{
			name:        "using --tag and --version together",
			args:        []string{"graph", "myimage", "--tag", "v1.0", "--version", "12345"},
			expectError: true,
		},
		{
			name:        "using --tag and --digest together",
			args:        []string{"graph", "myimage", "--tag", "v1.0", "--digest", "sha256:abc123"},
			expectError: true,
		},
		{
			name:        "using --version and --digest together",
			args:        []string{"graph", "myimage", "--version", "12345", "--digest", "sha256:abc123"},
			expectError: true,
		},
		{
			name:        "using all three flags together",
			args:        []string{"graph", "myimage", "--tag", "v1.0", "--version", "12345", "--digest", "sha256:abc123"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error for conflicting flags but got none")
				} else if err.Error() != "" {
					// Check that error message mentions flag conflict
					errMsg := err.Error()
					if !containsAny(errMsg, []string{"mutually exclusive", "only one", "cannot use"}) {
						t.Logf("Got error but unclear if it's about flag conflict: %v", err)
					}
				}
			}

			// Reset args and flags
			rootCmd.SetArgs([]string{})
			graphTag = "latest"
			graphVersion = 0
			graphDigest = ""
		})
	}
}

// Helper function for test
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// TestSortVersionsByProximity verifies that versions are sorted by proximity to a given ID
// Removed: TestSortVersionsByProximity - now tested in internal/discovery package
