package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestModulePathConsistency verifies that all Go files use the correct module path.
// This prevents accidental use of incorrect module paths (e.g., mhk vs mkoepf).
func TestModulePathConsistency(t *testing.T) {
	const expectedModule = "github.com/mkoepf/ghcrctl"
	const wrongModule = "github.com/mhk/ghcrctl"

	// Check go.mod
	goModContent, err := os.ReadFile("go.mod")
	require.NoError(t, err, "Failed to read go.mod")

	moduleRegex := regexp.MustCompile(`^module\s+(\S+)`)
	matches := moduleRegex.FindSubmatch(goModContent)
	require.GreaterOrEqual(t, len(matches), 2, "Could not find module declaration in go.mod")

	actualModule := string(matches[1])
	assert.Equal(t, expectedModule, actualModule, "go.mod has wrong module path")

	// Check all Go files for wrong import paths
	var filesWithWrongImport []string
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor directory and hidden directories
		if info.IsDir() && (info.Name() == "vendor" || strings.HasPrefix(info.Name(), ".")) {
			return filepath.SkipDir
		}

		// Only check .go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if strings.Contains(string(content), wrongModule) {
			filesWithWrongImport = append(filesWithWrongImport, path)
		}

		return nil
	})

	require.NoError(t, err, "Failed to walk directory")

	assert.Empty(t, filesWithWrongImport,
		"Found %d files with wrong module path %q (should be %q):\n  %s",
		len(filesWithWrongImport), wrongModule, expectedModule,
		strings.Join(filesWithWrongImport, "\n  "))
}
