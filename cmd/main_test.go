package cmd

import (
	"os"
	"testing"
)

// TestMain sets up the test environment before running tests.
func TestMain(m *testing.M) {
	// Run tests
	os.Exit(m.Run())
}
