package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/config"
)

func TestConfigOrgCommandExecution(t *testing.T) {
	// Cannot run in parallel - modifies shared config file
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

	// Execute org command with fresh command instance
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"config", "org", "testorg"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()

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
}

func TestConfigUserCommandExecution(t *testing.T) {
	// Cannot run in parallel - modifies shared config file
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

	// Execute user command with fresh command instance
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"config", "user", "testuser"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()

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
}

func TestConfigOrgCommand(t *testing.T) {
	t.Parallel()
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
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Execute command with fresh instance
			cmd := NewRootCmd()
			cmd.SetArgs(tt.args)

			// Capture output
			stdout := new(bytes.Buffer)
			stderr := new(bytes.Buffer)
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)

			err := cmd.Execute()

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
		})
	}
}

func TestConfigUserCommand(t *testing.T) {
	t.Parallel()
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
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Execute command with fresh instance
			cmd := NewRootCmd()
			cmd.SetArgs(tt.args)

			// Capture output
			stdout := new(bytes.Buffer)
			stderr := new(bytes.Buffer)
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)

			err := cmd.Execute()

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
		})
	}
}

func TestConfigShowWithNoConfigFile(t *testing.T) {
	// Cannot run in parallel - modifies HOME environment variable
	// Test config show when no config exists
	// This tests the empty config path (lines 27-30 in config.go)

	// Create temp directory for non-existent config
	tempDir, err := os.MkdirTemp("", "ghcrctl-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save current env vars and restore after test
	originalHome := os.Getenv("HOME")
	originalUserProfile := os.Getenv("USERPROFILE")
	originalConfig := os.Getenv("GHCRCTL_CONFIG")
	defer func() {
		os.Setenv("HOME", originalHome)
		if originalUserProfile != "" {
			os.Setenv("USERPROFILE", originalUserProfile)
		} else {
			os.Unsetenv("USERPROFILE")
		}
		if originalConfig != "" {
			os.Setenv("GHCRCTL_CONFIG", originalConfig)
		} else {
			os.Unsetenv("GHCRCTL_CONFIG")
		}
	}()

	// Clear GHCRCTL_CONFIG and set HOME/USERPROFILE to temp dir
	os.Unsetenv("GHCRCTL_CONFIG")
	os.Setenv("HOME", tempDir)
	os.Setenv("USERPROFILE", tempDir)

	// Execute config show with fresh instance - should show "No configuration found"
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"config", "show"})

	// Capture output
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err = cmd.Execute()

	if err != nil {
		t.Errorf("config show failed: %v", err)
	}
}

func TestConfigOrgAndUserToggle(t *testing.T) {
	// Cannot run in parallel - modifies shared config file
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
	cmd1 := NewRootCmd()
	cmd1.SetArgs([]string{"config", "org", "myorg"})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd1.SetOut(stdout)
	cmd1.SetErr(stderr)

	if err := cmd1.Execute(); err != nil {
		t.Errorf("Failed to set org: %v", err)
	}

	// Verify
	cfg := config.New()
	name, ownerType, _ := cfg.GetOwner()
	if name != "myorg" || ownerType != "org" {
		t.Errorf("Expected myorg (org), got %s (%s)", name, ownerType)
	}

	// Switch to user
	cmd2 := NewRootCmd()
	cmd2.SetArgs([]string{"config", "user", "myuser"})

	stdout = new(bytes.Buffer)
	stderr = new(bytes.Buffer)
	cmd2.SetOut(stdout)
	cmd2.SetErr(stderr)

	if err := cmd2.Execute(); err != nil {
		t.Errorf("Failed to set user: %v", err)
	}

	// Verify
	cfg = config.New()
	name, ownerType, _ = cfg.GetOwner()
	if name != "myuser" || ownerType != "user" {
		t.Errorf("Expected myuser (user), got %s (%s)", name, ownerType)
	}
}
