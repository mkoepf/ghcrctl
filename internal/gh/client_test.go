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

func TestValidateToken(t *testing.T) {
	// Note: This test requires mocking or a real token for integration testing
	// For now, we test that the function exists and handles context properly

	tests := []struct {
		name      string
		token     string
		skipTest  bool
		wantError bool
	}{
		{
			name:      "validation with mock token (will fail without real GitHub access)",
			token:     "ghp_fake_token_for_testing",
			skipTest:  true, // Skip unless we have integration test setup
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipTest {
				t.Skip("Skipping test that requires real GitHub API access")
			}

			client, err := NewClient(tt.token)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			ctx := context.Background()
			err = client.ValidateToken(ctx)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateTokenWithRealToken(t *testing.T) {
	// This test only runs if GITHUB_TOKEN is set in the environment
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	client, err := NewClient(token)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	err = client.ValidateToken(ctx)

	if err != nil {
		t.Errorf("Token validation failed with real token: %v", err)
	}
}
