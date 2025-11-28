package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/config"
)

func TestConfigOrgCommandExecution(t *testing.T) {
	// This test verifies the org command runs without error
	// It will modify the actual config file, but we'll restore it after

	// Save original config
	originalCfg := config.New()
	originalName, originalType, _ := originalCfg.GetOwner()

	// Cleanup function to restore original config
	defer func() {
		if originalName != "" && originalType != "" {
			_ = originalCfg.SetOwner(originalName, originalType)
		}
	}()

	// Execute org command
	rootCmd.SetArgs([]string{"config", "org", "testorg"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("config org command failed: %v", err)
	}

	// Verify the org was set
	cfg := config.New()
	name, ownerType, err := cfg.GetOwner()
	if err != nil {
		t.Errorf("Failed to get owner after setting: %v", err)
	}

	if name != "testorg" {
		t.Errorf("Expected owner name 'testorg', got '%s'", name)
	}

	if ownerType != "org" {
		t.Errorf("Expected owner type 'org', got '%s'", ownerType)
	}

	// Reset
	rootCmd.SetArgs([]string{})
}

func TestConfigUserCommandExecution(t *testing.T) {
	// This test verifies the user command runs without error
	// It will modify the actual config file, but we'll restore it after

	// Save original config
	originalCfg := config.New()
	originalName, originalType, _ := originalCfg.GetOwner()

	// Cleanup function to restore original config
	defer func() {
		if originalName != "" && originalType != "" {
			_ = originalCfg.SetOwner(originalName, originalType)
		}
	}()

	// Execute user command
	rootCmd.SetArgs([]string{"config", "user", "testuser"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("config user command failed: %v", err)
	}

	// Verify the user was set
	cfg := config.New()
	name, ownerType, err := cfg.GetOwner()
	if err != nil {
		t.Errorf("Failed to get owner after setting: %v", err)
	}

	if name != "testuser" {
		t.Errorf("Expected owner name 'testuser', got '%s'", name)
	}

	if ownerType != "user" {
		t.Errorf("Expected owner type 'user', got '%s'", ownerType)
	}

	// Reset
	rootCmd.SetArgs([]string{})
}

func TestConfigOrgCommand(t *testing.T) {
	// Test argument validation for org command

	tests := []struct {
		name      string
		args      []string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "missing org name",
			args:      []string{"config", "org"},
			wantError: true,
			errorMsg:  "accepts 1 arg(s), received 0",
		},
		{
			name:      "too many arguments",
			args:      []string{"config", "org", "org1", "org2"},
			wantError: true,
			errorMsg:  "accepts 1 arg(s), received 2",
		},
		{
			name:      "empty org name",
			args:      []string{"config", "org", ""},
			wantError: true,
			errorMsg:  "owner name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute command
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			// Reset for next test
			rootCmd.SetArgs([]string{})
		})
	}
}

func TestConfigUserCommand(t *testing.T) {
	// Test argument validation for user command

	tests := []struct {
		name      string
		args      []string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "missing user name",
			args:      []string{"config", "user"},
			wantError: true,
			errorMsg:  "accepts 1 arg(s), received 0",
		},
		{
			name:      "too many arguments",
			args:      []string{"config", "user", "user1", "user2"},
			wantError: true,
			errorMsg:  "accepts 1 arg(s), received 2",
		},
		{
			name:      "empty user name",
			args:      []string{"config", "user", ""},
			wantError: true,
			errorMsg:  "owner name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute command
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			// Reset for next test
			rootCmd.SetArgs([]string{})
		})
	}
}

func TestConfigShowWithNoConfigFile(t *testing.T) {
	// Test config show when no config exists
	// This tests the empty config path (lines 27-30 in config.go)

	// Create temp directory for non-existent config
	tempDir, err := os.MkdirTemp("", "ghcrctl-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save current HOME and USERPROFILE (for Windows) and restore after test
	originalHome := os.Getenv("HOME")
	originalUserProfile := os.Getenv("USERPROFILE")
	defer func() {
		os.Setenv("HOME", originalHome)
		if originalUserProfile != "" {
			os.Setenv("USERPROFILE", originalUserProfile)
		} else {
			os.Unsetenv("USERPROFILE")
		}
	}()

	// Set HOME and USERPROFILE to temp dir so config.New() uses it
	os.Setenv("HOME", tempDir)
	os.Setenv("USERPROFILE", tempDir)

	// Execute config show - should show "No configuration found"
	rootCmd.SetArgs([]string{"config", "show"})
	err = rootCmd.Execute()

	if err != nil {
		t.Errorf("config show failed: %v", err)
	}

	rootCmd.SetArgs([]string{})
}

func TestConfigOrgAndUserToggle(t *testing.T) {
	// Test switching between org and user
	// Save original config
	originalCfg := config.New()
	originalName, originalType, _ := originalCfg.GetOwner()

	defer func() {
		if originalName != "" && originalType != "" {
			_ = originalCfg.SetOwner(originalName, originalType)
		}
	}()

	// Set as org
	rootCmd.SetArgs([]string{"config", "org", "myorg"})
	if err := rootCmd.Execute(); err != nil {
		t.Errorf("Failed to set org: %v", err)
	}

	// Verify
	cfg := config.New()
	name, ownerType, _ := cfg.GetOwner()
	if name != "myorg" || ownerType != "org" {
		t.Errorf("Expected myorg (org), got %s (%s)", name, ownerType)
	}

	// Switch to user
	rootCmd.SetArgs([]string{"config", "user", "myuser"})
	if err := rootCmd.Execute(); err != nil {
		t.Errorf("Failed to set user: %v", err)
	}

	// Verify
	name, ownerType, _ = cfg.GetOwner()
	if name != "myuser" || ownerType != "user" {
		t.Errorf("Expected myuser (user), got %s (%s)", name, ownerType)
	}

	rootCmd.SetArgs([]string{})
}
