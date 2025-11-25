package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetOwner(t *testing.T) {
	// Create temporary config directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Test case 1: Config file doesn't exist
	cfg := NewWithPath(configPath)
	name, ownerType, err := cfg.GetOwner()
	if err != nil {
		t.Errorf("GetOwner() should not error when config doesn't exist, got %v", err)
	}
	if name != "" {
		t.Errorf("GetOwner() name should return empty string when config doesn't exist, got %s", name)
	}
	if ownerType != "" {
		t.Errorf("GetOwner() type should return empty string when config doesn't exist, got %s", ownerType)
	}

	// Test case 2: Config file exists with owner
	testConfig := "owner-name: testorg\nowner-type: org\n"
	err = os.WriteFile(configPath, []byte(testConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	name, ownerType, err = cfg.GetOwner()
	if err != nil {
		t.Errorf("GetOwner() error = %v, want nil", err)
	}
	if name != "testorg" {
		t.Errorf("GetOwner() name = %s, want testorg", name)
	}
	if ownerType != "org" {
		t.Errorf("GetOwner() type = %s, want org", ownerType)
	}

	// Test case 3: Config with user type
	testConfig = "owner-name: testuser\nowner-type: user\n"
	err = os.WriteFile(configPath, []byte(testConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	name, ownerType, err = cfg.GetOwner()
	if err != nil {
		t.Errorf("GetOwner() error = %v, want nil", err)
	}
	if name != "testuser" {
		t.Errorf("GetOwner() name = %s, want testuser", name)
	}
	if ownerType != "user" {
		t.Errorf("GetOwner() type = %s, want user", ownerType)
	}
}

func TestSetOwner(t *testing.T) {
	// Create temporary config directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := NewWithPath(configPath)

	// Test case 1: Set org owner when config doesn't exist
	err := cfg.SetOwner("myorg", "org")
	if err != nil {
		t.Errorf("SetOwner() error = %v, want nil", err)
	}

	// Verify it was written
	name, ownerType, err := cfg.GetOwner()
	if err != nil {
		t.Errorf("GetOwner() after SetOwner() error = %v", err)
	}
	if name != "myorg" {
		t.Errorf("GetOwner() name after SetOwner() = %s, want myorg", name)
	}
	if ownerType != "org" {
		t.Errorf("GetOwner() type after SetOwner() = %s, want org", ownerType)
	}

	// Test case 2: Update to user type
	err = cfg.SetOwner("myuser", "user")
	if err != nil {
		t.Errorf("SetOwner() second time error = %v, want nil", err)
	}

	name, ownerType, err = cfg.GetOwner()
	if err != nil {
		t.Errorf("GetOwner() after second SetOwner() error = %v", err)
	}
	if name != "myuser" {
		t.Errorf("GetOwner() name after second SetOwner() = %s, want myuser", name)
	}
	if ownerType != "user" {
		t.Errorf("GetOwner() type after second SetOwner() = %s, want user", ownerType)
	}

	// Test case 3: Invalid owner type should return error
	err = cfg.SetOwner("test", "invalid")
	if err == nil {
		t.Errorf("SetOwner() with invalid type should return error")
	}
}

func TestLoad(t *testing.T) {
	// Create temporary config directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := NewWithPath(configPath)

	// Test case 1: Load when file doesn't exist (should not error)
	err := cfg.Load()
	if err != nil {
		t.Errorf("Load() should not error when config doesn't exist, got %v", err)
	}

	// Test case 2: Load existing valid config
	testConfig := "owner-name: testuser\nowner-type: user\n"
	err = os.WriteFile(configPath, []byte(testConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	err = cfg.Load()
	if err != nil {
		t.Errorf("Load() error = %v, want nil", err)
	}

	name, ownerType, _ := cfg.GetOwner()
	if name != "testuser" {
		t.Errorf("After Load(), GetOwner() name = %s, want testuser", name)
	}
	if ownerType != "user" {
		t.Errorf("After Load(), GetOwner() type = %s, want user", ownerType)
	}
}

func TestSave(t *testing.T) {
	// Create temporary config directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := NewWithPath(configPath)

	// Test case: Save creates the config file
	err := cfg.SetOwner("savetest", "org")
	if err != nil {
		t.Errorf("SetOwner() error = %v, want nil", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Config file was not created at %s", configPath)
	}

	// Verify content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Config file is empty")
	}
}

func TestNew(t *testing.T) {
	// Test that New() creates a valid Config with default path
	cfg := New()
	if cfg == nil {
		t.Error("New() returned nil")
	}

	// Verify the path is set (we'll just check it's not empty)
	if cfg.path == "" {
		t.Error("New() created Config with empty path")
	}
}

func TestNewWithPath(t *testing.T) {
	tmpDir := t.TempDir()
	customPath := filepath.Join(tmpDir, "custom.yaml")

	cfg := NewWithPath(customPath)
	if cfg == nil {
		t.Error("NewWithPath() returned nil")
	}

	if cfg.path != customPath {
		t.Errorf("NewWithPath() path = %s, want %s", cfg.path, customPath)
	}

	// Test that it can use the custom path
	err := cfg.SetOwner("customtest", "org")
	if err != nil {
		t.Errorf("SetOwner() with custom path error = %v", err)
	}

	// Verify file was created at custom path
	if _, err := os.Stat(customPath); os.IsNotExist(err) {
		t.Errorf("Config file was not created at custom path %s", customPath)
	}
}

func TestGetOwnerWithEnvVars(t *testing.T) {
	// Create temporary config directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create config with file-based owner
	testConfig := "owner-name: fileorg\nowner-type: org\n"
	err := os.WriteFile(configPath, []byte(testConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg := NewWithPath(configPath)

	// Test case 1: Env vars override config file
	t.Setenv("GHCRCTL_OWNER", "envorg")
	t.Setenv("GHCRCTL_OWNER_TYPE", "org")

	name, ownerType, err := cfg.GetOwner()
	if err != nil {
		t.Errorf("GetOwner() with env vars error = %v, want nil", err)
	}
	if name != "envorg" {
		t.Errorf("GetOwner() with env vars name = %s, want envorg", name)
	}
	if ownerType != "org" {
		t.Errorf("GetOwner() with env vars type = %s, want org", ownerType)
	}

	// Test case 2: Only GHCRCTL_OWNER set, should default to "user" type
	os.Unsetenv("GHCRCTL_OWNER_TYPE")
	t.Setenv("GHCRCTL_OWNER", "envuser")

	name, ownerType, err = cfg.GetOwner()
	if err != nil {
		t.Errorf("GetOwner() with only GHCRCTL_OWNER error = %v, want nil", err)
	}
	if name != "envuser" {
		t.Errorf("GetOwner() with only GHCRCTL_OWNER name = %s, want envuser", name)
	}
	if ownerType != "user" {
		t.Errorf("GetOwner() with only GHCRCTL_OWNER type = %s, want user (default)", ownerType)
	}

	// Test case 3: No env vars, should fall back to config file
	os.Unsetenv("GHCRCTL_OWNER")
	os.Unsetenv("GHCRCTL_OWNER_TYPE")

	name, ownerType, err = cfg.GetOwner()
	if err != nil {
		t.Errorf("GetOwner() fallback to file error = %v, want nil", err)
	}
	if name != "fileorg" {
		t.Errorf("GetOwner() fallback to file name = %s, want fileorg", name)
	}
	if ownerType != "org" {
		t.Errorf("GetOwner() fallback to file type = %s, want org", ownerType)
	}

	// Test case 4: Both GHCRCTL_OWNER and GHCRCTL_OWNER_TYPE set to user
	t.Setenv("GHCRCTL_OWNER", "anotheruser")
	t.Setenv("GHCRCTL_OWNER_TYPE", "user")

	name, ownerType, err = cfg.GetOwner()
	if err != nil {
		t.Errorf("GetOwner() with user type env vars error = %v, want nil", err)
	}
	if name != "anotheruser" {
		t.Errorf("GetOwner() with user type name = %s, want anotheruser", name)
	}
	if ownerType != "user" {
		t.Errorf("GetOwner() with user type = %s, want user", ownerType)
	}
}

func TestGetOwnerEnvVarsNoConfigFile(t *testing.T) {
	// Create temporary config directory but don't create config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := NewWithPath(configPath)

	// Test env vars work even when config file doesn't exist
	t.Setenv("GHCRCTL_OWNER", "envonly")
	t.Setenv("GHCRCTL_OWNER_TYPE", "org")

	name, ownerType, err := cfg.GetOwner()
	if err != nil {
		t.Errorf("GetOwner() with env vars but no file error = %v, want nil", err)
	}
	if name != "envonly" {
		t.Errorf("GetOwner() with env vars but no file name = %s, want envonly", name)
	}
	if ownerType != "org" {
		t.Errorf("GetOwner() with env vars but no file type = %s, want org", ownerType)
	}
}
