package cmd

import (
	"bytes"
	"testing"
)

func TestListImagesCmd_RequiresPackageName(t *testing.T) {
	rootCmd := NewRootCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	rootCmd.SetArgs([]string{"list", "images"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for missing package name")
	}
}

func TestListImagesCmd_HasFlags(t *testing.T) {
	rootCmd := NewRootCmd()
	imagesCmd, _, err := rootCmd.Find([]string{"list", "images"})
	if err != nil {
		t.Fatalf("Failed to find list images command: %v", err)
	}

	// Check --json flag exists
	jsonFlag := imagesCmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Error("expected --json flag")
	}

	// Check --flat flag exists (replaced --tree, default is now tree)
	flatFlag := imagesCmd.Flags().Lookup("flat")
	if flatFlag == nil {
		t.Error("expected --flat flag")
	}

	// Check -o flag exists
	outputFlag := imagesCmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("expected -o/--output flag")
	}
}

func TestListImagesCmd_InvalidOutputFormat(t *testing.T) {
	rootCmd := NewRootCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	rootCmd.SetArgs([]string{"list", "images", "owner/test-package", "-o", "invalid"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid output format")
	}
}

func TestListImagesCmd_HasVersionAndDigestFlags(t *testing.T) {
	rootCmd := NewRootCmd()
	imagesCmd, _, err := rootCmd.Find([]string{"list", "images"})
	if err != nil {
		t.Fatalf("Failed to find list images command: %v", err)
	}

	// Check --version flag exists
	versionFlag := imagesCmd.Flags().Lookup("version")
	if versionFlag == nil {
		t.Error("expected --version flag")
	}

	// Check --digest flag exists
	digestFlag := imagesCmd.Flags().Lookup("digest")
	if digestFlag == nil {
		t.Error("expected --digest flag")
	}
}

func TestListImagesCmd_MutuallyExclusiveFlags(t *testing.T) {
	rootCmd := NewRootCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	rootCmd.SetArgs([]string{"list", "images", "owner/test-package", "--version", "123", "--digest", "sha256:abc"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when both --version and --digest are specified")
	}
}
