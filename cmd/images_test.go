package cmd

import (
	"bytes"
	"testing"
)

func TestImagesCmd_RequiresPackageName(t *testing.T) {
	cmd := newImagesCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing package name")
	}
}

func TestImagesCmd_HasFlags(t *testing.T) {
	cmd := newImagesCmd()

	// Check --json flag exists
	jsonFlag := cmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Error("expected --json flag")
	}

	// Check --flat flag exists (replaced --tree, default is now tree)
	flatFlag := cmd.Flags().Lookup("flat")
	if flatFlag == nil {
		t.Error("expected --flat flag")
	}

	// Check -o flag exists
	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("expected -o/--output flag")
	}
}

func TestImagesCmd_InvalidOutputFormat(t *testing.T) {
	cmd := newImagesCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"owner/test-package", "-o", "invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid output format")
	}
}
