package gh

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

			// Check error and token
			if tt.wantError {
				require.Error(t, err)
				assert.EqualError(t, err, tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantToken, token)
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
				require.Error(t, err)
				assert.EqualError(t, err, tt.errorMsg)
				assert.Nil(t, client)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
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
			require.NoError(t, err)

			ctx := context.Background()
			packages, err := client.ListPackages(ctx, tt.owner, tt.ownerType)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, packages)
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
			require.NoError(t, err)

			ctx := context.Background()
			_, err = client.GetVersionIDByDigest(ctx, tt.owner, tt.ownerType, tt.pkg, tt.digest)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetOwnerType(t *testing.T) {
	tests := []struct {
		name      string
		owner     string
		wantError bool
		errMsg    string
	}{
		{
			name:      "empty owner",
			owner:     "",
			wantError: true,
			errMsg:    "owner cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient("ghp_fake_token")
			require.NoError(t, err)

			ctx := context.Background()
			ownerType, err := client.GetOwnerType(ctx, tt.owner)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.True(t, ownerType == "user" || ownerType == "org", "expected 'user' or 'org', got '%s'", ownerType)
			}
		})
	}
}

func TestListPackageVersions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		owner     string
		ownerType string
		pkg       string
		wantError bool
		errMsg    string
	}{
		{
			name:      "empty owner",
			owner:     "",
			ownerType: "org",
			pkg:       "test-package",
			wantError: true,
			errMsg:    "owner cannot be empty",
		},
		{
			name:      "invalid owner type",
			owner:     "test",
			ownerType: "invalid",
			pkg:       "test-package",
			wantError: true,
			errMsg:    "owner type must be",
		},
		{
			name:      "empty package name",
			owner:     "test",
			ownerType: "org",
			pkg:       "",
			wantError: true,
			errMsg:    "package name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient("ghp_fake_token")
			require.NoError(t, err)

			ctx := context.Background()
			_, err = client.ListPackageVersions(ctx, tt.owner, tt.ownerType, tt.pkg)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
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
			require.NoError(t, err)

			ctx := context.Background()
			_, err = client.GetVersionTags(ctx, tt.owner, tt.ownerType, tt.pkg, tt.versionID)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
