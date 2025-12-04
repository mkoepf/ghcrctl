package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListImagesCmd_RequiresPackageName(t *testing.T) {
	rootCmd := NewRootCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	rootCmd.SetArgs([]string{"list", "images"})

	err := rootCmd.Execute()
	assert.Error(t, err, "expected error for missing package name")
}

func TestListImagesCmd_HasFlags(t *testing.T) {
	rootCmd := NewRootCmd()
	imagesCmd, _, err := rootCmd.Find([]string{"list", "images"})
	require.NoError(t, err, "Failed to find list images command")

	// Check --json flag exists
	jsonFlag := imagesCmd.Flags().Lookup("json")
	assert.NotNil(t, jsonFlag, "expected --json flag")

	// Check --flat flag exists (replaced --tree, default is now tree)
	flatFlag := imagesCmd.Flags().Lookup("flat")
	assert.NotNil(t, flatFlag, "expected --flat flag")

	// Check -o flag exists
	outputFlag := imagesCmd.Flags().Lookup("output")
	assert.NotNil(t, outputFlag, "expected -o/--output flag")
}

func TestListImagesCmd_InvalidOutputFormat(t *testing.T) {
	rootCmd := NewRootCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	rootCmd.SetArgs([]string{"list", "images", "owner/test-package", "-o", "invalid"})

	err := rootCmd.Execute()
	assert.Error(t, err, "expected error for invalid output format")
}

func TestListImagesCmd_HasVersionAndDigestFlags(t *testing.T) {
	rootCmd := NewRootCmd()
	imagesCmd, _, err := rootCmd.Find([]string{"list", "images"})
	require.NoError(t, err, "Failed to find list images command")

	// Check --version flag exists
	versionFlag := imagesCmd.Flags().Lookup("version")
	assert.NotNil(t, versionFlag, "expected --version flag")

	// Check --digest flag exists
	digestFlag := imagesCmd.Flags().Lookup("digest")
	assert.NotNil(t, digestFlag, "expected --digest flag")
}

func TestListImagesCmd_MutuallyExclusiveFlags(t *testing.T) {
	rootCmd := NewRootCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	rootCmd.SetArgs([]string{"list", "images", "owner/test-package", "--version", "123", "--digest", "sha256:abc"})

	err := rootCmd.Execute()
	assert.Error(t, err, "expected error when both --version and --digest are specified")
}
