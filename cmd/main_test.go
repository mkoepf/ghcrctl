package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/config"
)

// TestMain sets up the test environment before running tests.
// It creates a shared test config file that all integration tests use,
// preventing parallel tests from interfering with each other via the config file.
func TestMain(m *testing.M) {
	// Create a temp directory for test config
	tempDir, err := os.MkdirTemp("", "ghcrctl-test-*")
	if err != nil {
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	// Set up config path via environment variable
	configPath := filepath.Join(tempDir, "config.yaml")
	os.Setenv("GHCRCTL_CONFIG", configPath)
	defer os.Unsetenv("GHCRCTL_CONFIG")

	// Initialize config with test owner
	cfg := config.New()
	if err := cfg.SetOwner("mkoepf", "user"); err != nil {
		os.Exit(1)
	}

	// Run tests
	os.Exit(m.Run())
}
