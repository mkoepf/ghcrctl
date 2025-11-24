package gh

import (
	"context"
	"os"
	"testing"
)

func TestGetToken(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		setEnv    bool
		wantToken string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "token present in environment",
			envValue:  "ghp_test_token_12345",
			setEnv:    true,
			wantToken: "ghp_test_token_12345",
			wantError: false,
		},
		{
			name:      "token missing from environment",
			envValue:  "",
			setEnv:    false,
			wantToken: "",
			wantError: true,
			errorMsg:  "GITHUB_TOKEN environment variable not set",
		},
		{
			name:      "token is empty string",
			envValue:  "",
			setEnv:    true,
			wantToken: "",
			wantError: true,
			errorMsg:  "GITHUB_TOKEN environment variable is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env var
			originalToken := os.Getenv("GITHUB_TOKEN")
			defer func() {
				if originalToken != "" {
					os.Setenv("GITHUB_TOKEN", originalToken)
				} else {
					os.Unsetenv("GITHUB_TOKEN")
				}
			}()

			// Set up test environment
			if tt.setEnv {
				os.Setenv("GITHUB_TOKEN", tt.envValue)
			} else {
				os.Unsetenv("GITHUB_TOKEN")
			}

			// Call function
			token, err := GetToken()

			// Check error
			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			// Check token
			if token != tt.wantToken {
				t.Errorf("Expected token '%s', got '%s'", tt.wantToken, token)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid token",
			token:     "ghp_test_token_12345",
			wantError: false,
		},
		{
			name:      "empty token",
			token:     "",
			wantError: true,
			errorMsg:  "token cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.token)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error '%s', got '%s'", tt.errorMsg, err.Error())
				}
				if client != nil {
					t.Errorf("Expected nil client on error, got non-nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if client == nil {
					t.Errorf("Expected non-nil client, got nil")
				}
			}
		})
	}
}

func TestListPackages(t *testing.T) {
	// Test input validation for ListPackages

	tests := []struct {
		name      string
		owner     string
		ownerType string
		wantError bool
	}{
		{
			name:      "empty owner",
			owner:     "",
			ownerType: "org",
			wantError: true,
		},
		{
			name:      "invalid owner type",
			owner:     "test",
			ownerType: "invalid",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient("ghp_fake_token")
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			ctx := context.Background()
			packages, err := client.ListPackages(ctx, tt.owner, tt.ownerType)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if packages == nil {
					t.Error("Expected non-nil packages slice")
				}
			}
		})
	}
}

func TestSortPackages(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "already sorted",
			input:    []string{"alpha", "beta", "gamma"},
			expected: []string{"alpha", "beta", "gamma"},
		},
		{
			name:     "reverse order",
			input:    []string{"gamma", "beta", "alpha"},
			expected: []string{"alpha", "beta", "gamma"},
		},
		{
			name:     "random order",
			input:    []string{"zeta", "alpha", "delta", "beta"},
			expected: []string{"alpha", "beta", "delta", "zeta"},
		},
		{
			name:     "single element",
			input:    []string{"only"},
			expected: []string{"only"},
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "duplicates",
			input:    []string{"beta", "alpha", "beta", "alpha"},
			expected: []string{"alpha", "alpha", "beta", "beta"},
		},
		{
			name:     "case sensitive sorting",
			input:    []string{"Zebra", "apple", "Banana"},
			expected: []string{"Banana", "Zebra", "apple"}, // Uppercase comes before lowercase in ASCII
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying the test case
			packages := make([]string, len(tt.input))
			copy(packages, tt.input)

			// Sort the packages
			sortPackages(packages)

			// Verify the result
			if len(packages) != len(tt.expected) {
				t.Fatalf("Length mismatch: got %d, want %d", len(packages), len(tt.expected))
			}

			for i := range packages {
				if packages[i] != tt.expected[i] {
					t.Errorf("Position %d: got %s, want %s", i, packages[i], tt.expected[i])
				}
			}
		})
	}
}

func TestGetVersionIDByDigest(t *testing.T) {
	tests := []struct {
		name      string
		owner     string
		ownerType string
		pkg       string
		digest    string
		wantError bool
	}{
		{
			name:      "empty owner",
			owner:     "",
			ownerType: "org",
			pkg:       "test-package",
			digest:    "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantError: true,
		},
		{
			name:      "invalid owner type",
			owner:     "test",
			ownerType: "invalid",
			pkg:       "test-package",
			digest:    "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantError: true,
		},
		{
			name:      "empty package name",
			owner:     "test",
			ownerType: "org",
			pkg:       "",
			digest:    "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantError: true,
		},
		{
			name:      "empty digest",
			owner:     "test",
			ownerType: "org",
			pkg:       "test-package",
			digest:    "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient("ghp_fake_token")
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			ctx := context.Background()
			versionID, err := client.GetVersionIDByDigest(ctx, tt.owner, tt.ownerType, tt.pkg, tt.digest)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				// Can't verify actual version ID without real API call
				_ = versionID
			}
		})
	}
}

func TestGetVersionTags(t *testing.T) {
	tests := []struct {
		name      string
		owner     string
		ownerType string
		pkg       string
		versionID int64
		wantError bool
	}{
		{
			name:      "empty owner",
			owner:     "",
			ownerType: "org",
			pkg:       "test-package",
			versionID: 12345,
			wantError: true,
		},
		{
			name:      "invalid owner type",
			owner:     "test",
			ownerType: "invalid",
			pkg:       "test-package",
			versionID: 12345,
			wantError: true,
		},
		{
			name:      "empty package name",
			owner:     "test",
			ownerType: "org",
			pkg:       "",
			versionID: 12345,
			wantError: true,
		},
		{
			name:      "zero version ID",
			owner:     "test",
			ownerType: "org",
			pkg:       "test-package",
			versionID: 0,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient("ghp_fake_token")
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			ctx := context.Background()
			tags, err := client.GetVersionTags(ctx, tt.owner, tt.ownerType, tt.pkg, tt.versionID)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				// Can't verify actual tags without real API call
				_ = tags
			}
		})
	}
}

